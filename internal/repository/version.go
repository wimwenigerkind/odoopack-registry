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

// Upsert writes version under its natural key (addon_id, version) in one
// statement, so two writers racing on the same version cannot both insert, and
// loads the stored row, including its id, back into version. An empty storage
// key, content hash or size keeps the stored one.
func (r *AddonVersionRepository) Upsert(version *models.AddonVersion) error {
	return r.db.Clauses(
		clause.OnConflict{
			Columns: []clause.Column{{Name: "addon_id"}, {Name: "version"}},
			DoUpdates: clause.Assignments(map[string]any{
				"ref_type":     gorm.Expr("excluded.ref_type"),
				"ref_value":    gorm.Expr("excluded.ref_value"),
				"status":       gorm.Expr("excluded.status"),
				"storage_key":  gorm.Expr("COALESCE(NULLIF(excluded.storage_key, ''), addon_versions.storage_key)"),
				"content_hash": gorm.Expr("COALESCE(NULLIF(excluded.content_hash, ''), addon_versions.content_hash)"),
				"size_bytes":   gorm.Expr("COALESCE(NULLIF(excluded.size_bytes, 0), addon_versions.size_bytes)"),
				"updated_at":   gorm.Expr("excluded.updated_at"),
			}),
		},
		clause.Returning{},
	).Create(version).Error
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

type ManifestMeta struct {
	Depends         []string
	ManifestVersion string
	Summary         string
	License         string
	Category        string
	Author          string
	Website         string
}

func (r *AddonVersionRepository) SetManifestMeta(id uuid.UUID, meta ManifestMeta) error {
	return r.db.Model(&models.AddonVersion{}).Where("id = ?", id).
		Select("depends", "manifest_version", "summary", "license", "category", "author", "website").
		Updates(models.AddonVersion{
			Depends:         meta.Depends,
			ManifestVersion: meta.ManifestVersion,
			Summary:         meta.Summary,
			License:         meta.License,
			Category:        meta.Category,
			Author:          meta.Author,
			Website:         meta.Website,
		}).Error
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
