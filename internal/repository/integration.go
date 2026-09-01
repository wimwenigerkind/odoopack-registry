package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type IntegrationRepository struct {
	db *gorm.DB
}

func NewIntegrationRepository(db *gorm.DB) *IntegrationRepository {
	return &IntegrationRepository{db: db}
}

// Create FIXME: encrypt tokens before storing
func (r *IntegrationRepository) Create(it *models.OAuthIntegration) error {
	return r.db.Create(it).Error
}

func (r *IntegrationRepository) ListByOwner(ownerID uuid.UUID) ([]models.OAuthIntegration, error) {
	var out []models.OAuthIntegration
	err := r.db.Where("owner_id = ?", ownerID).Order("created_at DESC").Find(&out).Error
	return out, err
}

func (r *IntegrationRepository) GetByID(id uuid.UUID) (*models.OAuthIntegration, error) {
	var it models.OAuthIntegration
	err := r.db.Where("id = ?", id).First(&it).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &it, nil
}

func (r *IntegrationRepository) Delete(id, ownerID uuid.UUID) error {
	res := r.db.Where("id = ? AND owner_id = ?", id, ownerID).Delete(&models.OAuthIntegration{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *IntegrationRepository) UpdateTokens(it *models.OAuthIntegration) error {
	return r.db.Model(&models.OAuthIntegration{}).
		Where("id = ?", it.ID).
		Updates(map[string]any{
			"access_token":  it.AccessToken,
			"refresh_token": it.RefreshToken,
			"expires_at":    it.ExpiresAt,
		}).Error
}

func (r *IntegrationRepository) RefreshTokensWithLock(ctx context.Context, id uuid.UUID, refresh func(*models.OAuthIntegration) (bool, error)) (*models.OAuthIntegration, error) {
	var it models.OAuthIntegration
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", id).First(&it).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		changed, err := refresh(&it)
		if err != nil {
			return err
		}
		if !changed {
			return nil
		}
		return tx.Model(&models.OAuthIntegration{}).
			Where("id = ?", it.ID).
			Updates(map[string]any{
				"access_token":  it.AccessToken,
				"refresh_token": it.RefreshToken,
				"expires_at":    it.ExpiresAt,
			}).Error
	})
	if err != nil {
		return nil, err
	}
	return &it, nil
}
