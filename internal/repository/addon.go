package repository

import (
	"errors"

	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"gorm.io/gorm"
)

var ErrNotFound = errors.New("repository: record not found")

type AddonRepository struct {
	db *gorm.DB
}

func NewAddonRepository(db *gorm.DB) *AddonRepository {
	return &AddonRepository{db: db}
}

func (r *AddonRepository) Create(addon *models.Addon) error {
	return r.db.Create(addon).Error
}

func (r *AddonRepository) Update(addon *models.Addon) error {
	return r.db.Model(addon).Select("git_url", "default_branch", "visibility", "integration_id").Updates(map[string]any{
		"git_url":        addon.GitURL,
		"default_branch": addon.DefaultBranch,
		"visibility":     addon.Visibility,
		"integration_id": addon.IntegrationID,
	}).Error
}

func (r *AddonRepository) GetByID(id uuid.UUID) (*models.Addon, error) {
	var addon models.Addon
	err := r.db.Preload("Versions").Preload("Owner").Preload("Integration").Where("id = ?", id).First(&addon).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &addon, nil
}

func (r *AddonRepository) GetByName(name string) (*models.Addon, error) {
	var addon models.Addon
	err := r.db.Preload("Versions").Preload("Owner").Where("name = ?", name).First(&addon).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &addon, nil
}

func (r *AddonRepository) ListByOwnerAndRepoPath(ownerID uuid.UUID, repoPath string) ([]models.Addon, error) {
	var addons []models.Addon
	pattern := "%" + repoPath + "%"
	err := r.db.Where("owner_id = ? AND git_url LIKE ?", ownerID, pattern).Find(&addons).Error
	return addons, err
}

func (r *AddonRepository) ListVisibleTo(userID *uuid.UUID, isAdmin bool, nameFilter string) ([]models.Addon, error) {
	var addons []models.Addon
	q := r.db.Preload("Versions").Preload("Owner")
	if nameFilter != "" {
		q = q.Where("name = ?", nameFilter)
	}
	switch {
	case isAdmin:
	case userID == nil:
		q = q.Where("visibility = ?", models.VisibilityPublic)
	default:
		q = q.Where(
			"visibility = ? OR owner_id = ? OR EXISTS (SELECT 1 FROM group_memberships gm JOIN group_addon_accesses gaa ON gaa.group_id = gm.group_id WHERE gm.user_id = ? AND gaa.addon_id = addons.id)",
			models.VisibilityPublic, *userID, *userID,
		)
	}
	err := q.Find(&addons).Error
	return addons, err
}
