package database

import (
	"fmt"
	"log/slog"

	"github.com/wimwenigerkind/odoopack-registry/internal/crypto"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

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
func Migrate(db *gorm.DB, cipher *crypto.Cipher) error {
	if db == nil {
		return fmt.Errorf("database connection not established")
	}

	if err := hashLegacyApiTokens(db); err != nil {
		return fmt.Errorf("failed to hash legacy api tokens: %w", err)
	}

	err := db.AutoMigrate(
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
	)
	if err != nil {
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
