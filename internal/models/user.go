package models

import "github.com/google/uuid"

type User struct {
	Base
	Username   string     `json:"username"`
	Email      string     `gorm:"index" json:"email"`
	IsAdmin    bool       `gorm:"not null;default:false" json:"is_admin"`
	Identities []Identity `json:"identities,omitempty"`
}

type Identity struct {
	Base
	Subject  string    `gorm:"not null;uniqueIndex:idx_provider_subject" json:"-"`
	Provider string    `gorm:"not null;uniqueIndex:idx_provider_subject" json:"provider"`
	UserID   uuid.UUID `gorm:"not null;index" json:"user_id"`
	User     User      `json:"-"`
}
