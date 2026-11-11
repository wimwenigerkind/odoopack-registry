//go:build integration

package database

import (
	"strings"
	"testing"

	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"gorm.io/gorm"
)

func TestMigrateIndexesAddonModuleNames(t *testing.T) {
	db := testDB(t)
	for run := range 2 {
		if err := Migrate(db, nil); err != nil {
			t.Fatalf("run %d: %v", run+1, err)
		}
	}
	if !db.Migrator().HasIndex(&models.Addon{}, "idx_addons_module_name") {
		t.Fatal("idx_addons_module_name missing")
	}

	// With sequential scans priced out, the planner takes the index for the
	// expression the repository queries with, or the two have drifted apart.
	var plan []string
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SET LOCAL enable_seqscan = off").Error; err != nil {
			return err
		}
		return tx.Raw(`EXPLAIN SELECT name FROM addons WHERE translate(addons.name, '/-', '__') IN ('acme_essentials', 'acme_warranty')`).Scan(&plan).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(plan, "\n"), "idx_addons_module_name") {
		t.Errorf("plan does not use idx_addons_module_name:\n%s", strings.Join(plan, "\n"))
	}
}
