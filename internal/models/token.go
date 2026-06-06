package models

import (
	"time"

	"github.com/google/uuid"
)

type ApiToken struct {
	Base
	UserID     uuid.UUID  `gorm:"not null;index" json:"user_id"`
	Name       string     `gorm:"not null" json:"name"`
	Token      string     `gorm:"not null;uniqueIndex" json:"token"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}
