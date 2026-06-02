package models

import "github.com/google/uuid"

type User struct {
	Base
	Username   string     `json:"username,omitempty"`
	Identities []Identity `json:"identities,omitempty"`
}

type Identity struct {
	Base
	Subject       string    `gorm:"not null;uniqueIndex:idx_provider_subject" json:"-"`
	Email         string    `gorm:"not null" json:"email"`
	Provider      string    `gorm:"not null;uniqueIndex:idx_provider_subject" json:"provider"`
	UserID        uuid.UUID `gorm:"not null;index" json:"user_id"`
	User          User      `json:"-"`
	EmailVerified bool      `gorm:"-" json:"-"`
}
