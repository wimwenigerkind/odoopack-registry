package repository

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"gorm.io/gorm"
)

type ApiTokenRepository struct {
	db *gorm.DB
}

func NewApiTokenRepository(db *gorm.DB) *ApiTokenRepository {
	return &ApiTokenRepository{db: db}
}

// Create FIXME: encrypt token before storing
func (r *ApiTokenRepository) Create(token *models.ApiToken) error {
	return r.db.Create(token).Error
}

func (r *ApiTokenRepository) ListByUser(userID uuid.UUID) ([]models.ApiToken, error) {
	var tokens []models.ApiToken
	err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&tokens).Error
	return tokens, err
}

func (r *ApiTokenRepository) GetByToken(plain string) (*models.ApiToken, error) {
	var token models.ApiToken
	err := r.db.Where("token = ?", plain).First(&token).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &token, nil
}

func (r *ApiTokenRepository) Delete(id, userID uuid.UUID) error {
	res := r.db.Where("id = ? AND user_id = ?", id, userID).Delete(&models.ApiToken{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *ApiTokenRepository) TouchLastUsed(id uuid.UUID, now time.Time) error {
	cutoff := now.Add(-1 * time.Minute)
	return r.db.Model(&models.ApiToken{}).
		Where("id = ? AND (last_used_at IS NULL OR last_used_at < ?)", id, cutoff).
		Update("last_used_at", now).Error
}
