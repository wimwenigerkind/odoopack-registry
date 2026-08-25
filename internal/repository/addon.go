package repository

import (
	"errors"

	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"gorm.io/gorm"
)

var ErrNotFound = errors.New("repository: record not found")
var ErrConflict = errors.New("repository: conflict")

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
	err := r.db.Model(addon).Select("name", "subpath", "visibility").Updates(map[string]any{
		"name":       addon.Name,
		"subpath":    addon.Subpath,
		"visibility": addon.Visibility,
	}).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return ErrConflict
	}
	return err
}

func (r *AddonRepository) GetByID(id uuid.UUID) (*models.Addon, error) {
	var addon models.Addon
	err := r.db.
		Preload("Versions").
		Preload("Repo").
		Preload("Repo.Owner").
		Preload("Repo.Integration").
		Where("id = ?", id).First(&addon).Error
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
	err := r.db.
		Preload("Versions").
		Preload("Repo").
		Preload("Repo.Owner").
		Where("name = ?", name).First(&addon).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &addon, nil
}

func (r *AddonRepository) Delete(id uuid.UUID) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("addon_id = ?", id).Delete(&models.GroupAddonAccess{}).Error; err != nil {
			return err
		}
		versionIDs := tx.Model(&models.AddonVersion{}).Select("id").Where("addon_id = ?", id)
		if err := tx.Where("addon_version_id IN (?)", versionIDs).Delete(&models.AddonVersionReadme{}).Error; err != nil {
			return err
		}
		if err := tx.Where("addon_id = ?", id).Delete(&models.AddonVersion{}).Error; err != nil {
			return err
		}
		res := tx.Where("id = ?", id).Delete(&models.Addon{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrNotFound
		}
		return nil
	})
}

func (r *AddonRepository) ListByRepo(repoID uuid.UUID) ([]models.Addon, error) {
	var addons []models.Addon
	err := r.db.Where("repo_id = ?", repoID).Order("name").Find(&addons).Error
	return addons, err
}

func (r *AddonRepository) ListByOwnerAndRepoPath(ownerID uuid.UUID, repoPath string) ([]models.Addon, error) {
	var addons []models.Addon
	pattern := "%" + repoPath + "%"
	err := r.db.
		Preload("Repo").
		Joins("Repo").
		Where(`"Repo".owner_id = ?`, ownerID).
		Where(`"Repo".git_url LIKE ?`, pattern).
		Find(&addons).Error
	return addons, err
}

func (r *AddonRepository) ListVisibleTo(userID *uuid.UUID, isAdmin bool, nameFilter string) ([]models.Addon, error) {
	var addons []models.Addon
	q := r.db.
		Preload("Versions").
		Preload("Repo").
		Preload("Repo.Owner").
		Joins("Repo")

	if nameFilter != "" {
		q = q.Where("addons.name = ?", nameFilter)
	}

	switch {
	case isAdmin:
	case userID == nil:
		q = q.Where("addons.visibility = ?", models.VisibilityPublic)
	default:
		groupGrant := r.db.
			Table("group_memberships AS gm").
			Select("1").
			Joins("JOIN group_addon_accesses gaa ON gaa.group_id = gm.group_id").
			Where("gm.user_id = ?", *userID).
			Where("gaa.addon_id = addons.id")

		q = q.Where(
			r.db.Where("addons.visibility = ?", models.VisibilityPublic).
				Or(`"Repo".owner_id = ?`, *userID).
				Or("EXISTS (?)", groupGrant),
		)
	}

	err := q.Find(&addons).Error
	return addons, err
}
