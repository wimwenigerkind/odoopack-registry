//go:build integration

package database

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
)

func TestMigrateDedupesActiveSyncJobs(t *testing.T) {
	db := testDB(t)
	if err := Migrate(db, nil); err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasIndex(&models.SyncJob{}, "idx_sync_jobs_active") {
		t.Fatal("idx_sync_jobs_active missing")
	}
	if err := db.Migrator().DropIndex(&models.SyncJob{}, "idx_sync_jobs_active"); err != nil {
		t.Fatal(err)
	}

	addonID := uuid.New()
	base := time.Now()
	for i, status := range []models.SyncJobStatus{
		models.SyncJobPending, models.SyncJobPending, models.SyncJobRunning,
		models.SyncJobRunning, models.SyncJobFailed, models.SyncJobFailed,
	} {
		job := models.SyncJob{AddonID: addonID, Status: status, RunAfter: base}
		job.CreatedAt = base.Add(time.Duration(i) * time.Second)
		if err := db.Create(&job).Error; err != nil {
			t.Fatal(err)
		}
	}

	if err := Migrate(db, nil); err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasIndex(&models.SyncJob{}, "idx_sync_jobs_active") {
		t.Error("idx_sync_jobs_active not recreated")
	}
	for status, want := range map[models.SyncJobStatus]int64{
		models.SyncJobPending: 1,
		models.SyncJobRunning: 1,
		models.SyncJobFailed:  2,
	} {
		var n int64
		if err := db.Model(&models.SyncJob{}).Where("addon_id = ? AND status = ?", addonID, status).Count(&n).Error; err != nil {
			t.Fatal(err)
		}
		if n != want {
			t.Errorf("%s jobs = %d, want %d", status, n, want)
		}
	}
}
