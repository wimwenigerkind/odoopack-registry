//go:build integration

// Package testdb gives every integration test a schema of its own in the
// Postgres database in TEST_DATABASE_DSN, so tests neither see each other's
// rows nor depend on what an earlier run left behind.
package testdb

import (
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// admin creates and drops the schemas. It is shared by the tests of a package
// and lives as long as the test binary.
var admin = sync.OnceValues(func() (*gorm.DB, error) {
	return gorm.Open(postgres.Open(os.Getenv("TEST_DATABASE_DSN")), &gorm.Config{})
})

// DSN creates an empty schema that is dropped when t ends and returns
// TEST_DATABASE_DSN with its search_path set to that schema, which pgx sends
// as a runtime parameter on every connection of a pool opened with it. It
// skips t when TEST_DATABASE_DSN is not set.
func DSN(t testing.TB) string {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN not set")
	}
	db, err := admin()
	if err != nil {
		t.Fatal(err)
	}
	schema := "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := db.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Errorf("drop schema %s: %v", schema, err)
		}
	})
	return withSearchPath(dsn, schema)
}

// Open connects to a new schema from DSN with connect, which is
// database.Connect; testdb cannot import database because database's own
// tests use it. The pool is closed before the schema is dropped.
func Open(t testing.TB, connect func(dsn string) (*gorm.DB, error)) *gorm.DB {
	t.Helper()
	db, err := connect(DSN(t))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

// withSearchPath adds search_path to a DSN in either form pgx accepts: a URL
// or keyword/value pairs.
func withSearchPath(dsn, schema string) string {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		return dsn + sep + "search_path=" + schema
	}
	return dsn + " search_path=" + schema
}
