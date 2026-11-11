//go:build integration

package repository_test

import (
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/database"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"github.com/wimwenigerkind/odoopack-registry/internal/repository"
)

func TestSyncTriggerClaimAllowsOnePerInterval(t *testing.T) {
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
	triggers := repository.NewSyncTriggerRepository(db)

	// Replicas asking at once: exactly one gets the window.
	var allowed atomic.Int32
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			wait, err := triggers.Claim(addon.ID, time.Minute)
			if err != nil {
				t.Error(err)
			}
			if wait == 0 {
				allowed.Add(1)
			}
		})
	}
	wg.Wait()
	if n := allowed.Load(); n != 1 {
		t.Fatalf("%d claims allowed, want 1", n)
	}

	wait, err := triggers.Claim(addon.ID, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if wait <= 50*time.Second || wait > time.Minute {
		t.Errorf("wait = %s, want just under 1m", wait)
	}

	// Once the last trigger is older than the interval, the next one passes.
	if err := db.Exec("UPDATE sync_triggers SET triggered_at = now() - interval '2 minutes' WHERE addon_id = ?", addon.ID).Error; err != nil {
		t.Fatal(err)
	}
	if wait, err := triggers.Claim(addon.ID, time.Minute); err != nil || wait != 0 {
		t.Fatalf("after the interval: wait = %s, err = %v, want allowed", wait, err)
	}

	repos := repository.NewAddonRepository(db)
	if err := repos.Delete(addon.ID); err != nil {
		t.Fatal(err)
	}
	var left int64
	db.Model(&models.SyncTrigger{}).Where("addon_id = ?", addon.ID).Count(&left)
	if left != 0 {
		t.Errorf("deleting the addon left %d sync trigger rows", left)
	}
}
