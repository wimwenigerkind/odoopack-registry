//go:build integration

package database

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/crypto"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"github.com/wimwenigerkind/odoopack-registry/internal/repository"
	"gorm.io/gorm"
)

// integrationTestDB needs a disposable Postgres database in
// TEST_DATABASE_DSN; the tests drop and recreate the integration and repo
// tables there.
func integrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN not set")
	}
	db, err := Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropTable(&models.Repo{}, &models.OAuthIntegration{}); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db, nil); err != nil {
		t.Fatal(err)
	}
	return db
}

func storedTokens(t *testing.T, db *gorm.DB, id uuid.UUID) (access, refresh string) {
	t.Helper()
	var row models.OAuthIntegration
	if err := db.Where("id = ?", id).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	return row.AccessToken, row.RefreshToken
}

func readTokens(t *testing.T, repo *repository.IntegrationRepository, id uuid.UUID) (access, refresh string) {
	t.Helper()
	it, err := repo.RefreshTokensWithLock(context.Background(), id, func(*models.OAuthIntegration) (bool, error) {
		return false, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return it.AccessToken, it.RefreshToken
}

func TestMigrateSealsPlaintextIntegrationTokens(t *testing.T) {
	db := integrationTestDB(t)
	c := testCipher(t, 1)

	legacy := models.OAuthIntegration{Provider: "github", OwnerID: uuid.New(), AccountName: "acme", AccessToken: "gho_acme_access", RefreshToken: "ghr_acme_refresh"}
	noRefresh := models.OAuthIntegration{Provider: "forgejo", OwnerID: uuid.New(), AccountName: "acme", AccessToken: "8a1f0c2e9b7d4a6f3e5c1b0a9d8e7f6a5b4c3d2e"}
	for _, it := range []*models.OAuthIntegration{&legacy, &noRefresh} {
		if err := db.Create(it).Error; err != nil {
			t.Fatal(err)
		}
	}

	if err := Migrate(db, nil); err != nil {
		t.Fatal(err)
	}
	if access, _ := storedTokens(t, db, legacy.ID); access != legacy.AccessToken {
		t.Fatalf("migrating without a key changed the token to %q", access)
	}

	if err := Migrate(db, c); err != nil {
		t.Fatal(err)
	}
	firstAccess, firstRefresh := storedTokens(t, db, legacy.ID)
	if !crypto.LooksSealed(firstAccess) || !crypto.LooksSealed(firstRefresh) {
		t.Fatalf("tokens not sealed: %q, %q", firstAccess, firstRefresh)
	}
	if _, refresh := storedTokens(t, db, noRefresh.ID); refresh != "" {
		t.Errorf("empty refresh token became %q", refresh)
	}

	if err := Migrate(db, c); err != nil {
		t.Fatal(err)
	}
	if access, refresh := storedTokens(t, db, legacy.ID); access != firstAccess || refresh != firstRefresh {
		t.Error("a second run sealed the tokens again")
	}

	repo := repository.NewIntegrationRepository(db, c)
	if access, refresh := readTokens(t, repo, legacy.ID); access != legacy.AccessToken || refresh != legacy.RefreshToken {
		t.Errorf("read %q, %q, want the tokens written before the migration", access, refresh)
	}
	if access, _ := readTokens(t, repo, noRefresh.ID); access != noRefresh.AccessToken {
		t.Errorf("read %q, want %q", access, noRefresh.AccessToken)
	}

	// A key that does not match leaves the sealed tokens alone instead of
	// sealing them a second time.
	if err := Migrate(db, testCipher(t, 2)); err != nil {
		t.Fatal(err)
	}
	if access, _ := storedTokens(t, db, legacy.ID); access != firstAccess {
		t.Error("a run with another key rewrote a sealed token")
	}
}

func TestIntegrationRepositorySealsTokens(t *testing.T) {
	db := integrationTestDB(t)
	c := testCipher(t, 1)
	repo := repository.NewIntegrationRepository(db, c)
	ctx := context.Background()

	it := &models.OAuthIntegration{Provider: "github", OwnerID: uuid.New(), AccountName: "acme", AccessToken: "gho_one", RefreshToken: "ghr_one"}
	if err := repo.Create(it); err != nil {
		t.Fatal(err)
	}
	if it.AccessToken != "gho_one" {
		t.Errorf("create changed the caller's token to %q", it.AccessToken)
	}
	if access, _ := storedTokens(t, db, it.ID); !crypto.LooksSealed(access) {
		t.Fatalf("stored %q", access)
	}

	it.AccessToken, it.RefreshToken = "gho_two", ""
	if err := repo.UpdateTokens(it); err != nil {
		t.Fatal(err)
	}
	if access, refresh := readTokens(t, repo, it.ID); access != "gho_two" || refresh != "" {
		t.Errorf("read %q, %q after update", access, refresh)
	}

	if _, err := repo.RefreshTokensWithLock(ctx, it.ID, func(cur *models.OAuthIntegration) (bool, error) {
		cur.AccessToken, cur.RefreshToken = "gho_three", "ghr_three"
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}
	if access, refresh := readTokens(t, repo, it.ID); access != "gho_three" || refresh != "ghr_three" {
		t.Errorf("read %q, %q after refresh", access, refresh)
	}
	if _, refresh := storedTokens(t, db, it.ID); !crypto.LooksSealed(refresh) {
		t.Errorf("refreshed token stored as %q", refresh)
	}

	got, err := repo.GetByID(it.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.AccessToken != "" || got.RefreshToken != "" || got.AccountName != "acme" {
		t.Errorf("GetByID returned %+v", got)
	}
	list, err := repo.ListByOwner(it.OwnerID)
	if err != nil || len(list) != 1 || list[0].AccessToken != "" {
		t.Errorf("ListByOwner returned %+v, %v", list, err)
	}

	owner := &models.User{Username: "acme"}
	if err := db.Create(owner).Error; err != nil {
		t.Fatal(err)
	}
	r := &models.Repo{GitURL: "https://github.com/acme/addons.git", DefaultBranch: "main", OwnerID: owner.ID, IntegrationID: &it.ID}
	if err := db.Create(r).Error; err != nil {
		t.Fatal(err)
	}
	loaded, err := repository.NewRepoRepository(db).GetByID(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Integration == nil || loaded.Integration.AccessToken != "" {
		t.Errorf("preloaded integration %+v", loaded.Integration)
	}

	// The relink flow reads by account, which must keep working with a key
	// that no longer opens the stored tokens.
	lost := repository.NewIntegrationRepository(db, testCipher(t, 2))
	if _, err := lost.GetByOwnerProviderAccount(it.OwnerID, "github", "acme"); err != nil {
		t.Errorf("lookup with another key: %v", err)
	}
	if _, err := lost.RefreshTokensWithLock(ctx, it.ID, func(*models.OAuthIntegration) (bool, error) { return false, nil }); err == nil {
		t.Error("opened tokens with another key")
	}
}
