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
	Name       string         `gorm:"uniqueIndex;not null" json:"name"`
	RepoID     uuid.UUID      `gorm:"type:uuid;not null;index" json:"repo_id"`
	Repo       *Repo          `json:"repo,omitempty"`
	Subpath    string         `json:"subpath,omitempty"`
	Visibility Visibility     `gorm:"not null;default:public" json:"visibility"`
	Versions   []AddonVersion `json:"versions,omitempty"`
}

type AddonVersion struct {
	Base
	AddonID         uuid.UUID     `gorm:"type:uuid;index:idx_addon_ref,unique;not null" json:"-"`
	Version         string        `gorm:"index:idx_addon_ref,unique;not null" json:"version"`
	RefType         RefType       `gorm:"not null" json:"ref_type"`
	RefValue        string        `gorm:"not null" json:"ref_value"`
	Status          VersionStatus `gorm:"not null;default:pending" json:"status"`
	StorageKey      string        `json:"-"`
	ContentHash     string        `json:"content_hash,omitempty"`
	SizeBytes       int64         `json:"size_bytes,omitempty"`
	BuildError      string        `json:"build_error,omitempty"`
	BuiltAt         *time.Time    `json:"built_at,omitempty"`
	Depends         []string      `gorm:"serializer:json" json:"depends,omitempty"`
	ManifestVersion string        `json:"manifest_version,omitempty"`
	Series          string        `gorm:"-" json:"series,omitempty"`
	IsLatest        bool          `gorm:"-" json:"is_latest"`
}

type AddonVersionReadme struct {
	AddonVersionID uuid.UUID `gorm:"type:uuid;primaryKey" json:"-"`
	HTML           string    `gorm:"type:text" json:"html"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type SyncJobStatus string

const (
	SyncJobPending SyncJobStatus = "pending"
	SyncJobRunning SyncJobStatus = "running"
	SyncJobFailed  SyncJobStatus = "failed"
)

type SyncJob struct {
	Base
	AddonID       uuid.UUID     `gorm:"type:uuid;not null;index" json:"addon_id"`
	Name          string        `json:"name"`
	GitURL        string        `json:"git_url"`
	DefaultBranch string        `json:"default_branch"`
	Subpath       string        `json:"subpath"`
	Trigger       string        `json:"trigger"`
	IntegrationID *uuid.UUID    `gorm:"type:uuid" json:"integration_id,omitempty"`
	Status        SyncJobStatus `gorm:"not null;default:pending;index" json:"status"`
	Attempts      int           `gorm:"not null;default:0" json:"attempts"`
	MaxAttempts   int           `gorm:"not null;default:3" json:"max_attempts"`
	RunAfter      time.Time     `gorm:"index" json:"run_after"`
	LockedAt      *time.Time    `json:"locked_at,omitempty"`
	LockedBy      string        `json:"locked_by,omitempty"`
	LastError     string        `json:"last_error,omitempty"`
}
