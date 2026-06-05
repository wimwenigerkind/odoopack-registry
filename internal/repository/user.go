package repository

import (
	"errors"

	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) GetByID(id uuid.UUID) (*models.User, error) {
	var user models.User
	err := r.db.Preload("Identities").Where("id = ?", id).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) List() ([]models.User, error) {
	var users []models.User
	err := r.db.Order("email").Find(&users).Error
	return users, err
}

func (r *UserRepository) GetByIdentity(provider, subject string) (*models.User, error) {
	var identity models.Identity
	err := r.db.Where("provider = ? AND subject = ?", provider, subject).First(&identity).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return r.GetByID(identity.UserID)
}

func (r *UserRepository) CreateWithIdentity(user *models.User, identity *models.Identity) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			return err
		}
		identity.UserID = user.ID
		return tx.Create(identity).Error
	})
}
