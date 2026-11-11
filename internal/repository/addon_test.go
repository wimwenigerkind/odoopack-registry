package repository

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// dryRunRepo builds queries for Postgres without a database: DryRun stops
// gorm before it executes, and sql.Open only dials on first use, which the
// disabled ping never triggers. The callback records each generated query;
// subqueries pass through it as well, before the statement that embeds them.
func dryRunRepo(t *testing.T) (*AddonRepository, *[]string) {
	t.Helper()
	db, err := gorm.Open(postgres.Open("host=db.invalid"), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	var queries []string
	err = db.Callback().Query().After("gorm:query").Register("test:capture", func(tx *gorm.DB) {
		queries = append(queries, tx.Explain(tx.Statement.SQL.String(), tx.Statement.Vars...))
	})
	if err != nil {
		t.Fatal(err)
	}
	return NewAddonRepository(db), &queries
}

func TestVisibleQueriesByMode(t *testing.T) {
	user := uuid.New()
	for _, tc := range []struct {
		name    string
		userID  *uuid.UUID
		isAdmin bool
		mode    string
		want    string
		notWant string
	}{
		{name: "anonymous public", mode: "public", want: `addons.visibility = 'public'`, notWant: "FALSE"},
		{name: "anonymous private", mode: "private", want: "FALSE", notWant: "addons.visibility"},
		{name: "signed in private", userID: &user, mode: "private", want: `addons.visibility = 'public' OR "Repo".owner_id = '` + user.String() + `'`, notWant: "FALSE"},
		{name: "admin private", isAdmin: true, mode: "private", notWant: "addons.visibility"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, queries := dryRunRepo(t)
			outer := func(name string, run func() error) string {
				*queries = nil
				if err := run(); err != nil {
					t.Fatalf("%s: %v", name, err)
				}
				return (*queries)[len(*queries)-1]
			}
			sqls := map[string]string{
				"ListVisibleTo": outer("ListVisibleTo", func() error {
					_, err := repo.ListVisibleTo(tc.userID, tc.isAdmin, tc.mode, "")
					return err
				}),
				"VisibleReadyVersions": outer("VisibleReadyVersions", func() error {
					_, err := repo.VisibleReadyVersions(tc.userID, tc.isAdmin, tc.mode)
					return err
				}),
				"VisibleQuery": outer("VisibleQuery", func() error {
					var page []models.Addon
					return repo.VisibleQuery(tc.userID, tc.isAdmin, tc.mode, AddonSearch{Series: "17.0"}).Find(&page).Error
				}),
			}
			for name, sql := range sqls {
				if tc.want != "" && !strings.Contains(sql, tc.want) {
					t.Errorf("%s lacks %q: %s", name, tc.want, sql)
				}
				if strings.Contains(sql, tc.notWant) {
					t.Errorf("%s contains %q: %s", name, tc.notWant, sql)
				}
			}
		})
	}
}
