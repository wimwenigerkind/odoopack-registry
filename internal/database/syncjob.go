package database

import "gorm.io/gorm"

// dedupeActiveSyncJobs keeps the oldest pending and the oldest running job per
// addon and deletes the rest, so that AutoMigrate can create the unique index
// idx_sync_jobs_active on a database that queued duplicates before it existed.
// A deleted duplicate loses nothing: the job that stays syncs every ref.
func dedupeActiveSyncJobs(db *gorm.DB) error {
	if !db.Migrator().HasTable("sync_jobs") {
		return nil
	}
	return db.Exec(`DELETE FROM sync_jobs WHERE id IN (
	SELECT id FROM (
		SELECT id, row_number() OVER (PARTITION BY addon_id, status ORDER BY created_at, id) AS n
		FROM sync_jobs WHERE status IN ('pending', 'running')
	) ranked WHERE n > 1
)`).Error
}
