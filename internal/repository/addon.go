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

func (r *AddonRepository) GetByID(id uuid.UUID) (*models.Addon, error) {
	var addon models.Addon
	err := r.db.Preload("Versions").Preload("Owner").Where("id = ?", id).First(&addon).Error
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

func (r *AddonRepository) ListVisibleTo(userID *uuid.UUID, nameFilter string) ([]models.Addon, error) {
	var addons []models.Addon
	q := r.db.Preload("Versions").Preload("Owner")
	if nameFilter != "" {
		q = q.Where("name = ?", nameFilter)
	}
	if userID == nil {
		q = q.Where("visibility = ?", models.VisibilityPublic)
	} else {
		q = q.Where("visibility = ? OR owner_id = ?", models.VisibilityPublic, *userID)
	}
	err := q.Find(&addons).Error
	return addons, err
}
