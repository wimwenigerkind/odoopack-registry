package models

import (
	"time"

	"github.com/google/uuid"
)

// ApiToken keeps only the sha256 of the token; Prefix and LastFour are stored
// at creation because the hash cannot give them back.
type ApiToken struct {
	Base
	UserID     uuid.UUID  `gorm:"not null;index" json:"user_id"`
	Name       string     `gorm:"not null" json:"name"`
	TokenHash  string     `gorm:"not null;uniqueIndex" json:"-"`
	Prefix     string     `gorm:"not null" json:"prefix"`
	LastFour   string     `gorm:"not null" json:"last_four"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}
