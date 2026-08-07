package repository

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AddonVersionRepository struct {
	db *gorm.DB
}

func NewAddonVersionRepository(db *gorm.DB) *AddonVersionRepository {
	return &AddonVersionRepository{db: db}
}

func (r *AddonVersionRepository) Upsert(version *models.AddonVersion) error {
	var existing models.AddonVersion
	err := r.db.Where("addon_id = ? AND version = ?", version.AddonID, version.Version).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return r.db.Create(version).Error
	}
	if err != nil {
		return err
	}
	existing.RefType = version.RefType
	existing.RefValue = version.RefValue
	existing.Status = version.Status
	if version.StorageKey != "" {
		existing.StorageKey = version.StorageKey
	}
	if version.ContentHash != "" {
		existing.ContentHash = version.ContentHash
	}
	if version.SizeBytes != 0 {
		existing.SizeBytes = version.SizeBytes
	}
	*version = existing
	return r.db.Save(version).Error
}

func (r *AddonVersionRepository) Get(addonID uuid.UUID, version string) (*models.AddonVersion, error) {
	var av models.AddonVersion
	err := r.db.Where("addon_id = ? AND version = ?", addonID, version).First(&av).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &av, nil
}

func (r *AddonVersionRepository) UpdateStatus(id uuid.UUID, status models.VersionStatus, buildErr string) error {
	return r.db.Model(&models.AddonVersion{}).Where("id = ?", id).Updates(map[string]any{
		"status":      status,
		"build_error": buildErr,
	}).Error
}

func (r *AddonVersionRepository) Delete(id uuid.UUID) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("addon_version_id = ?", id).Delete(&models.AddonVersionReadme{}).Error; err != nil {
			return err
		}
		res := tx.Where("id = ?", id).Delete(&models.AddonVersion{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrNotFound
		}
		return nil
	})
}

func (r *AddonVersionRepository) SetReadme(versionID uuid.UUID, html string) error {
	if html == "" {
		return r.db.Where("addon_version_id = ?", versionID).Delete(&models.AddonVersionReadme{}).Error
	}
	readme := &models.AddonVersionReadme{
		AddonVersionID: versionID,
		HTML:           html,
		UpdatedAt:      time.Now(),
	}
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "addon_version_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"html", "updated_at"}),
	}).Create(readme).Error
}

func (r *AddonVersionRepository) GetReadme(versionID uuid.UUID) (*models.AddonVersionReadme, error) {
	var readme models.AddonVersionReadme
	err := r.db.Where("addon_version_id = ?", versionID).First(&readme).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &readme, nil
}

func (r *AddonVersionRepository) SetReady(id uuid.UUID, storageKey, contentHash string, sizeBytes int64) error {
	now := time.Now()
	return r.db.Model(&models.AddonVersion{}).Where("id = ?", id).Updates(map[string]any{
		"status":       models.StatusReady,
		"storage_key":  storageKey,
		"content_hash": contentHash,
		"size_bytes":   sizeBytes,
		"built_at":     now,
		"build_error":  "",
	}).Error
}
