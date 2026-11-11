package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"log/slog"

	"github.com/wimwenigerkind/odoopack-registry/internal/crypto"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// migrationLockKey is the Postgres advisory lock key that serializes
// migrations: "odoopack" in ASCII, so it does not collide with other users of
// advisory locks in a shared database by accident.
const migrationLockKey int64 = 0x6f646f6f7061636b

var schema = []any{
	&models.User{},
	&models.Identity{},
	&models.OAuthIntegration{},
	&models.Repo{},
	&models.Addon{},
	&models.AddonVersion{},
	&models.AddonVersionReadme{},
	&models.SyncJob{},
	&models.Group{},
	&models.GroupMembership{},
	&models.GroupAddonAccess{},
	&models.ApiToken{},
}

func Connect(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		return nil, err
	}
	return db, nil
}

// Migrate brings the schema up to date. cipher seals integration tokens
// written before they were encrypted; nil skips that step, which is only right
// when no provider allows git integration and no key is configured.
//
// Concurrent callers, such as the migrate init containers of several web pods,
// serialize on a session-level advisory lock; every step is idempotent, so a
// caller that waited finds nothing left to do. The lock is taken on one pooled
// connection that also runs every step, because a session-level lock belongs to
// its connection and gorm would otherwise hand out others. A transaction-scoped
// lock would need one transaction around all steps, which commit separately on
// purpose.
func Migrate(db *gorm.DB, cipher *crypto.Cipher) error {
	if db == nil {
		return fmt.Errorf("database connection not established")
	}
	return db.Connection(func(conn *gorm.DB) (err error) {
		if err := conn.Exec("SELECT pg_advisory_lock(?)", migrationLockKey).Error; err != nil {
			return fmt.Errorf("failed to take the migration lock: %w", err)
		}
		defer func() {
			if unlockErr := conn.Exec("SELECT pg_advisory_unlock(?)", migrationLockKey).Error; unlockErr != nil {
				discardConn(conn)
				err = errors.Join(err, fmt.Errorf("failed to release the migration lock: %w", unlockErr))
			}
		}()
		return migrate(conn, cipher)
	})
}

// discardConn makes the pool close the connection instead of reusing it, which
// ends the session and with it a lock that could not be released.
func discardConn(conn *gorm.DB) {
	if c, ok := conn.Statement.ConnPool.(*sql.Conn); ok {
		_ = c.Raw(func(any) error { return driver.ErrBadConn })
	}
}

func migrate(db *gorm.DB, cipher *crypto.Cipher) error {
	if err := hashLegacyApiTokens(db); err != nil {
		return fmt.Errorf("failed to hash legacy api tokens: %w", err)
	}
	if err := dedupeActiveSyncJobs(db); err != nil {
		return fmt.Errorf("failed to dedupe sync jobs: %w", err)
	}

	if err := db.AutoMigrate(schema...); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	if cipher != nil {
		if err := sealIntegrationTokens(db, cipher); err != nil {
			return fmt.Errorf("failed to seal integration tokens: %w", err)
		}
	}

	slog.Info("database migrations completed")
	return nil
}

// SchemaReady reports whether a migration has created every table and none is
// running right now. The shared lock conflicts only with the exclusive one
// Migrate holds, so readers never wait on each other.
func SchemaReady(ctx context.Context, db *gorm.DB) (ready bool, err error) {
	err = db.WithContext(ctx).Connection(func(conn *gorm.DB) error {
		var locked bool
		if err := conn.Raw("SELECT pg_try_advisory_lock_shared(?)", migrationLockKey).Scan(&locked).Error; err != nil {
			return err
		}
		if !locked {
			return nil
		}
		defer func() {
			if err := conn.Exec("SELECT pg_advisory_unlock_shared(?)", migrationLockKey).Error; err != nil {
				discardConn(conn)
			}
		}()
		m := conn.Migrator()
		for _, model := range schema {
			if !m.HasTable(model) {
				return nil
			}
		}
		ready = true
		return nil
	})
	return ready, err
}
