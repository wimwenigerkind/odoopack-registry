package repository

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SyncTriggerRepository struct {
	db *gorm.DB
}

func NewSyncTriggerRepository(db *gorm.DB) *SyncTriggerRepository {
	return &SyncTriggerRepository{db: db}
}

// Claim records a sync request for the addon unless the last one is less than
// interval old; then it returns how long the caller has to wait. The upsert
// decides in one statement, so two replicas cannot both claim the same window,
// and it uses the database clock, which all replicas share.
func (r *SyncTriggerRepository) Claim(addonID uuid.UUID, interval time.Duration) (time.Duration, error) {
	secs := interval.Seconds()
	res := r.db.Exec(`
INSERT INTO sync_triggers (addon_id, triggered_at) VALUES (?, now())
ON CONFLICT (addon_id) DO UPDATE SET triggered_at = excluded.triggered_at
WHERE sync_triggers.triggered_at <= now() - make_interval(secs => ?)`, addonID, secs)
	if res.Error != nil {
		return 0, res.Error
	}
	if res.RowsAffected > 0 {
		return 0, nil
	}

	var wait float64
	err := r.db.Raw(`SELECT EXTRACT(EPOCH FROM triggered_at + make_interval(secs => ?) - now())
FROM sync_triggers WHERE addon_id = ?`, secs, addonID).Scan(&wait).Error
	if err != nil {
		return 0, err
	}
	// The window can close between the two statements.
	return max(time.Duration(wait*float64(time.Second)), time.Second), nil
}
