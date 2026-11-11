package worker

import (
	"time"

	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SyncJob struct {
	AddonID       uuid.UUID
	Name          string
	GitURL        string
	DefaultBranch string
	Subpath       string
	Trigger       string
	IntegrationID *uuid.UUID
}

type Queue struct {
	db          *gorm.DB
	maxAttempts int
}

func NewQueue(db *gorm.DB) *Queue {
	return &Queue{db: db, maxAttempts: 3}
}

func (q *Queue) Enqueue(job SyncJob) error {
	row := models.SyncJob{
		AddonID:       job.AddonID,
		Name:          job.Name,
		GitURL:        job.GitURL,
		DefaultBranch: job.DefaultBranch,
		Subpath:       job.Subpath,
		Trigger:       job.Trigger,
		IntegrationID: job.IntegrationID,
		Status:        models.SyncJobPending,
		MaxAttempts:   q.maxAttempts,
		RunAfter:      time.Now(),
	}
	// The unique index on active jobs turns a second pending job for the
	// addon into a no-op: the pending one has not listed the refs yet.
	return q.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error
}
