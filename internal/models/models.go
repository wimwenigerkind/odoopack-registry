package models

import (
	"time"

	"github.com/google/uuid"
)

type Visibility string

const (
	VisibilityPublic  Visibility = "public"
	VisibilityPrivate Visibility = "private"
)

type RefType string

const (
	RefTypeTag    RefType = "tag"
	RefTypeBranch RefType = "branch"
)

type VersionStatus string

const (
	StatusPending  VersionStatus = "pending"
	StatusBuilding VersionStatus = "building"
	StatusReady    VersionStatus = "ready"
	StatusFailed   VersionStatus = "failed"
)

type Addon struct {
	Base
	Name          string            `gorm:"uniqueIndex;not null" json:"name"`
	GitURL        string            `gorm:"not null" json:"git_url"`
	DefaultBranch string            `gorm:"default:main" json:"default_branch"`
	Visibility    Visibility        `gorm:"not null;default:public" json:"visibility"`
	OwnerID       uuid.UUID         `gorm:"type:uuid;not null;index" json:"owner_id"`
	Owner         *User             `json:"owner,omitempty"`
	IntegrationID *uuid.UUID        `gorm:"type:uuid;index" json:"integration_id,omitempty"`
	Integration   *OAuthIntegration `gorm:"foreignKey:IntegrationID" json:"integration,omitempty"`
	Versions      []AddonVersion    `json:"versions,omitempty"`
}

type AddonVersion struct {
	Base
	AddonID     uuid.UUID     `gorm:"type:uuid;index:idx_addon_ref,unique;not null" json:"-"`
	Version     string        `gorm:"index:idx_addon_ref,unique;not null" json:"version"`
	RefType     RefType       `gorm:"not null" json:"ref_type"`
	RefValue    string        `gorm:"not null" json:"ref_value"`
	Status      VersionStatus `gorm:"not null;default:pending" json:"status"`
	StorageKey  string        `json:"-"`
	ContentHash string        `json:"content_hash,omitempty"`
	SizeBytes   int64         `json:"size_bytes,omitempty"`
	BuildError  string        `json:"build_error,omitempty"`
	BuiltAt     *time.Time    `json:"built_at,omitempty"`
}
