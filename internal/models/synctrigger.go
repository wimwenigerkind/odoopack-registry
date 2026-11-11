package models

import (
	"time"

	"github.com/google/uuid"
)

// SyncTrigger holds when a user last asked for a sync of an addon. It lives
// in the database rather than in the web process so that every replica
// enforces the same interval; deleting the addon deletes it.
type SyncTrigger struct {
	AddonID     uuid.UUID `gorm:"type:uuid;primaryKey"`
	Addon       *Addon    `gorm:"constraint:OnDelete:CASCADE"`
	TriggeredAt time.Time `gorm:"not null"`
}
