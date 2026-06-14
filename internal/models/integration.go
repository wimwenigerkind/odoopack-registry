package models

import (
	"time"

	"github.com/google/uuid"
)

type OAuthIntegration struct {
	Base
	Provider     string     `gorm:"not null;index" json:"provider"`
	OwnerID      uuid.UUID  `gorm:"type:uuid;not null;index" json:"owner_id"`
	AccountName  string     `gorm:"not null" json:"account_name"`
	AccessToken  string     `gorm:"not null" json:"-"`
	RefreshToken string     `json:"-"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	Scope        string     `json:"scope,omitempty"`
}
