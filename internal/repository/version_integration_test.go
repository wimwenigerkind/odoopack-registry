//go:build integration

package repository_test

import (
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/database"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"github.com/wimwenigerkind/odoopack-registry/internal/repository"
)

// Two writers starting a build of the same version must end with one row that
// both of them hold the id of, and a ready version keeps its last good build.
func TestStartBuildConcurrentWritersShareOneRow(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN not set")
	}
	db, err := database.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db, nil); err != nil {
		t.Fatal(err)
	}
	user := models.User{Username: "acme"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	repo := models.Repo{GitURL: "https://git.example.com/acme/" + uuid.NewString() + ".git", OwnerID: user.ID}
	if err := db.Create(&repo).Error; err != nil {
		t.Fatal(err)
	}
	addon := models.Addon{Name: "acme_" + uuid.NewString(), RepoID: repo.ID}
	if err := db.Create(&addon).Error; err != nil {
		t.Fatal(err)
	}
	versions := repository.NewAddonVersionRepository(db)

	ready := &models.AddonVersion{AddonID: addon.ID, Version: "dev-main", RefType: models.RefTypeBranch,
		RefValue: "old", Status: models.StatusReady, StorageKey: "packages/old.zip", SizeBytes: 42}
	if err := db.Create(ready).Error; err != nil {
		t.Fatal(err)
	}

	ids := make([]uuid.UUID, 8)
	var wg sync.WaitGroup
	for i := range ids {
		wg.Go(func() {
			av := &models.AddonVersion{AddonID: addon.ID, Version: "dev-main", RefType: models.RefTypeBranch,
				RefValue: "new"}
			if err := versions.StartBuild(av); err != nil {
				t.Error(err)
			}
			ids[i] = av.ID
		})
	}
	wg.Wait()

	for i, id := range ids {
		if id != ready.ID {
			t.Errorf("writer %d got id %s, want the stored %s", i, id, ready.ID)
		}
	}
	var rows []models.AddonVersion
	if err := db.Where("addon_id = ?", addon.ID).Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	got := rows[0]
	if got.Status != models.StatusReady || got.RefValue != "old" || got.BuildingRef != "new" || got.StorageKey != "packages/old.zip" || got.SizeBytes != 42 {
		t.Errorf("row = %s %s building %q %q %d, want ready at old building new", got.Status, got.RefValue, got.BuildingRef, got.StorageKey, got.SizeBytes)
	}
}
