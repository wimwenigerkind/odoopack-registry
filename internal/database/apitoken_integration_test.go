//go:build integration

package database

import (
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/apitoken"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"github.com/wimwenigerkind/odoopack-registry/internal/repository"
	"gorm.io/gorm"
)

// legacyApiToken is api_tokens as it was before tokens were hashed.
type legacyApiToken struct {
	models.Base
	UserID     uuid.UUID `gorm:"not null;index"`
	Name       string    `gorm:"not null"`
	Token      string    `gorm:"not null;uniqueIndex"`
	LastUsedAt *time.Time
}

func (legacyApiToken) TableName() string { return "api_tokens" }

// testDB needs a disposable Postgres database in TEST_DATABASE_DSN; the tests
// drop and recreate api_tokens there.
func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN not set")
	}
	db, err := Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropTable("api_tokens"); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestMigrateCreatesApiTokens(t *testing.T) {
	db := testDB(t)
	for run := range 2 {
		if err := Migrate(db, nil); err != nil {
			t.Fatalf("run %d: %v", run+1, err)
		}
	}
	for _, column := range []string{"token_hash", "prefix", "last_four", "expires_at"} {
		if !db.Migrator().HasColumn("api_tokens", column) {
			t.Errorf("column %s missing", column)
		}
	}
	if db.Migrator().HasColumn("api_tokens", "token") {
		t.Error("plain token column exists")
	}
}

func TestMigrateHashesLegacyTokens(t *testing.T) {
	db := testDB(t)
	if err := db.AutoMigrate(&legacyApiToken{}); err != nil {
		t.Fatal(err)
	}
	const plain = "odp_fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	legacy := legacyApiToken{UserID: uuid.New(), Name: "acme laptop", Token: plain}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}

	for run := range 2 {
		if err := Migrate(db, nil); err != nil {
			t.Fatalf("run %d: %v", run+1, err)
		}
	}

	if db.Migrator().HasColumn("api_tokens", "token") {
		t.Error("plain token column still exists")
	}
	if !db.Migrator().HasIndex(&models.ApiToken{}, "TokenHash") {
		t.Error("token_hash has no index")
	}
	tok, err := repository.NewApiTokenRepository(db).GetByHash(apitoken.Hash(plain))
	if err != nil {
		t.Fatalf("legacy token not found by hash: %v", err)
	}
	if tok.ID != legacy.ID || tok.Prefix != "odp_fedc" || tok.LastFour != "3210" {
		t.Errorf("migrated token = %+v", tok)
	}

	fresh := models.ApiToken{UserID: uuid.New(), Name: "acme ci", TokenHash: apitoken.Hash("odp_new"), Prefix: "odp_new", LastFour: "_new"}
	if err := db.Create(&fresh).Error; err != nil {
		t.Errorf("create after migration: %v", err)
	}
	dup := fresh
	dup.ID = uuid.Nil
	if err := db.Create(&dup).Error; err == nil {
		t.Error("a second token with the same hash was accepted")
	}
	assertNotNull(t, db, "token_hash", "prefix", "last_four")
}

func assertNotNull(t *testing.T, db *gorm.DB, columns ...string) {
	t.Helper()
	types, err := db.Migrator().ColumnTypes("api_tokens")
	if err != nil {
		t.Fatal(err)
	}
	for _, ct := range types {
		for _, name := range columns {
			if ct.Name() == name {
				if nullable, ok := ct.Nullable(); !ok || nullable {
					t.Errorf("%s is nullable", name)
				}
			}
		}
	}
}
