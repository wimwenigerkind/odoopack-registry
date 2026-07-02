package models

import (
	"github.com/google/uuid"
)

type Repo struct {
	Base
	GitURL        string            `gorm:"uniqueIndex;not null" json:"git_url"`
	DefaultBranch string            `gorm:"default:main" json:"default_branch"`
	OwnerID       uuid.UUID         `gorm:"type:uuid;not null;index" json:"owner_id"`
	Owner         *User             `json:"owner,omitempty"`
	IntegrationID *uuid.UUID        `gorm:"type:uuid;index" json:"integration_id,omitempty"`
	Integration   *OAuthIntegration `gorm:"foreignKey:IntegrationID" json:"integration,omitempty"`
	Addons        []Addon           `json:"addons,omitempty"`
}
