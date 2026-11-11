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

// StartBuild records a build of version at its RefValue under the natural key
// (addon_id, version) in one statement, so two writers racing on the same
// version cannot both insert, and loads the stored row, including its id, back
// into version. The commit goes to building_ref. A version that is already
// ready keeps its status, ref and artifact, so it stays listed and
// installable at its last good build until SetReady swaps in the new one;
// any other version goes to building at the new ref.
func (r *AddonVersionRepository) StartBuild(version *models.AddonVersion) error {
	version.Status = models.StatusBuilding
	version.BuildingRef = version.RefValue
	return r.db.Clauses(
		clause.OnConflict{
			Columns: []clause.Column{{Name: "addon_id"}, {Name: "version"}},
			DoUpdates: clause.Assignments(map[string]any{
				"ref_type":     gorm.Expr("excluded.ref_type"),
				"ref_value":    gorm.Expr("CASE WHEN addon_versions.status = ? THEN addon_versions.ref_value ELSE excluded.ref_value END", models.StatusReady),
				"status":       gorm.Expr("CASE WHEN addon_versions.status = ? THEN addon_versions.status ELSE excluded.status END", models.StatusReady),
				"building_ref": gorm.Expr("excluded.building_ref"),
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

// FailBuild ends the build in progress with buildErr. A ready version stays
// ready at its last good build and only carries the error of the rebuild.
func (r *AddonVersionRepository) FailBuild(id uuid.UUID, buildErr string) error {
	return r.db.Model(&models.AddonVersion{}).Where("id = ?", id).Updates(map[string]any{
		"status":       gorm.Expr("CASE WHEN status = ? THEN status ELSE ? END", models.StatusReady, models.StatusFailed),
		"build_error":  buildErr,
		"building_ref": "",
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

type Build struct {
	RefValue    string
	StorageKey  string
	ContentHash string
	SizeBytes   int64
	// Manifest is nil when the archive has no manifest; the stored metadata
	// is kept then.
	Manifest *ManifestMeta
}

// SetReady publishes b in one UPDATE, so a client sees either the previous
// build or the new one, never the hash of one with the ref or depends of the
// other.
func (r *AddonVersionRepository) SetReady(id uuid.UUID, b Build) error {
	now := time.Now()
	columns := []string{"status", "ref_value", "storage_key", "content_hash", "size_bytes", "built_at", "build_error", "building_ref", "updated_at"}
	values := models.AddonVersion{
		Base:        models.Base{UpdatedAt: now},
		Status:      models.StatusReady,
		RefValue:    b.RefValue,
		StorageKey:  b.StorageKey,
		ContentHash: b.ContentHash,
		SizeBytes:   b.SizeBytes,
		BuiltAt:     &now,
	}
	if m := b.Manifest; m != nil {
		columns = append(columns, "depends", "manifest_version", "summary", "license", "category", "author", "website")
		values.Depends = m.Depends
		values.ManifestVersion = m.ManifestVersion
		values.Summary = m.Summary
		values.License = m.License
		values.Category = m.Category
		values.Author = m.Author
		values.Website = m.Website
	}
	return r.db.Model(&models.AddonVersion{}).Where("id = ?", id).Select(columns).Updates(values).Error
}
