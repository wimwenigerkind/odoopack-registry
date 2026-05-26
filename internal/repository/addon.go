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
	err := r.db.Preload("Versions").Where("id = ?", id).First(&addon).Error
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
	err := r.db.Preload("Versions").Where("name = ?", name).First(&addon).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &addon, nil
}

func (r *AddonRepository) List(nameFilter string) ([]models.Addon, error) {
	var addons []models.Addon
	q := r.db.Preload("Versions")
	if nameFilter != "" {
		q = q.Where("name = ?", nameFilter)
	}
	err := q.Find(&addons).Error
	return addons, err
}
