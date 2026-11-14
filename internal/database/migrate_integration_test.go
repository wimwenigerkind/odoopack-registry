//go:build integration

package database

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/apitoken"
	"github.com/wimwenigerkind/odoopack-registry/internal/crypto"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"github.com/wimwenigerkind/odoopack-registry/internal/repository"
	"github.com/wimwenigerkind/odoopack-registry/internal/testdb"
)

// countingHandler counts log records by message.
type countingHandler struct {
	mu     sync.Mutex
	counts map[string]int
}

func (h *countingHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *countingHandler) WithAttrs([]slog.Attr) slog.Handler       { return h }
func (h *countingHandler) WithGroup(string) slog.Handler            { return h }

func (h *countingHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.counts[r.Message]++
	return nil
}

func (h *countingHandler) count(msg string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.counts[msg]
}

func captureLogs(t *testing.T) *countingHandler {
	h := &countingHandler{counts: map[string]int{}}
	prev := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return h
}

// Several processes migrating the same fresh database at once, like the init
// containers of a scaled web deployment, must all succeed and do the work once.
// The sealing step logs only when it sealed something; without the lock every
// run would read the plaintext tokens before the first commit and seal again.
func TestMigrateConcurrently(t *testing.T) {
	dsn := testdb.DSN(t)
	db, err := Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if sqlDB, err := db.DB(); err == nil {
		defer func() { _ = sqlDB.Close() }()
	}
	c := testCipher(t, 1)

	if err := db.AutoMigrate(&legacyApiToken{}, &models.OAuthIntegration{}); err != nil {
		t.Fatal(err)
	}
	const plain = "odp_0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	legacyToken := legacyApiToken{UserID: uuid.New(), Name: "acme laptop", Token: plain}
	if err := db.Create(&legacyToken).Error; err != nil {
		t.Fatal(err)
	}
	integration := models.OAuthIntegration{Provider: "github", OwnerID: uuid.New(), AccountName: "acme", AccessToken: "gho_acme_access", RefreshToken: "ghr_acme_refresh"}
	if err := db.Create(&integration).Error; err != nil {
		t.Fatal(err)
	}

	logs := captureLogs(t)
	const runs = 8
	errs := make([]error, runs)
	var wg sync.WaitGroup
	for i := range runs {
		wg.Go(func() {
			// A connection pool per run, as each pod has its own.
			own, err := Connect(dsn)
			if err != nil {
				errs[i] = err
				return
			}
			if sqlDB, err := own.DB(); err == nil {
				defer func() { _ = sqlDB.Close() }()
			}
			errs[i] = Migrate(own, c)
		})
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("run %d: %v", i+1, err)
		}
	}

	if n := logs.count("database migrations completed"); n != runs {
		t.Errorf("%d runs completed, want %d", n, runs)
	}
	if n := logs.count("sealed integration tokens"); n != 1 {
		t.Errorf("tokens were sealed by %d runs, want 1", n)
	}
	for _, model := range schema {
		if !db.Migrator().HasTable(model) {
			t.Errorf("table for %T missing", model)
		}
	}
	if !db.Migrator().HasIndex(&models.SyncJob{}, "idx_sync_jobs_active") {
		t.Error("idx_sync_jobs_active missing")
	}
	if db.Migrator().HasColumn("api_tokens", "token") {
		t.Error("plain token column still exists")
	}
	if _, err := repository.NewApiTokenRepository(db).GetByHash(apitoken.Hash(plain)); err != nil {
		t.Errorf("legacy token not found by hash: %v", err)
	}
	access, _ := storedTokens(t, db, integration.ID)
	if !crypto.LooksSealed(access) {
		t.Fatalf("access token not sealed: %q", access)
	}

	if err := Migrate(db, c); err != nil {
		t.Fatal(err)
	}
	if n := logs.count("sealed integration tokens"); n != 1 {
		t.Errorf("a second run sealed tokens again")
	}
	if again, _ := storedTokens(t, db, integration.ID); again != access {
		t.Error("a second run rewrote the sealed token")
	}
}

// Migrate waits for a lock somebody else holds, and SchemaReady reports false
// while the schema is missing or a migration holds the lock.
func TestMigrateWaitsForTheLock(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	holder, err := sqlDB.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Close() }()
	if _, err := holder.ExecContext(ctx, "SELECT pg_advisory_lock($1)", migrationLockKey); err != nil {
		t.Fatal(err)
	}

	if ready, err := SchemaReady(ctx, db); err != nil || ready {
		t.Fatalf("SchemaReady while locked = %v, %v; want false", ready, err)
	}

	done := make(chan error, 1)
	go func() { done <- Migrate(db, nil) }()
	select {
	case err := <-done:
		t.Fatalf("Migrate returned while the lock was held: %v", err)
	case <-time.After(500 * time.Millisecond):
	}
	if db.Migrator().HasTable(&models.User{}) {
		t.Fatal("Migrate created tables while the lock was held")
	}

	if _, err := holder.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", migrationLockKey); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("Migrate did not finish after the lock was released")
	}

	if err := eventually(func() (bool, error) { return SchemaReady(ctx, db) }); err != nil {
		t.Fatalf("SchemaReady after migrating: %v", err)
	}

	// Migrate released its lock: another session can take it. A lock left on
	// one of db's pooled connections would stay held for good.
	err = eventually(func() (free bool, err error) {
		err = holder.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", migrationLockKey).Scan(&free)
		return free, err
	})
	if err != nil {
		t.Errorf("the migration lock is still held after Migrate returned: %v", err)
	}
	if _, err := holder.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", migrationLockKey); err != nil {
		t.Fatal(err)
	}
}

// eventually retries cond until it holds. The integration tests of other
// packages, run in parallel, migrate their own schemas under the same advisory
// lock, so a check that needs the lock free may find it taken for a moment.
func eventually(cond func() (bool, error)) error {
	deadline := time.Now().Add(30 * time.Second)
	for {
		ok, err := cond()
		if ok || err != nil {
			return err
		}
		if time.Now().After(deadline) {
			return errors.New("condition not met within 30s")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// WaitForSchema returns once the schema exists and stops with its context.
func TestWaitForSchema(t *testing.T) {
	db := testDB(t)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := WaitForSchema(ctx, db); err == nil {
		t.Fatal("WaitForSchema returned nil without a schema")
	}

	done := make(chan error, 1)
	go func() { done <- WaitForSchema(context.Background(), db) }()
	if err := Migrate(db, nil); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	// WaitForSchema backs off between checks, and a check that finds another
	// package's migration holding the lock waits for the next one.
	case <-time.After(20 * time.Second):
		t.Fatal("WaitForSchema did not return after the migration")
	}
}
