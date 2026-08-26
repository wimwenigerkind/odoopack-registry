package worker

import (
	"time"

	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"gorm.io/gorm"
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
	return q.db.Transaction(func(tx *gorm.DB) error {
		var pending int64
		if err := tx.Model(&models.SyncJob{}).
			Where("addon_id = ? AND status = ?", job.AddonID, models.SyncJobPending).
			Count(&pending).Error; err != nil {
			return err
		}
		if pending > 0 {
			return nil
		}
		return tx.Create(&row).Error
	})
}
