package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/crypto"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrNoCipher = errors.New("repository: integrations.encryption_key is not configured")

// IntegrationRepository seals the tokens on every write and opens them in
// RefreshTokensWithLock, the one read that hands them out, so callers only
// ever see plaintext. The other reads leave the tokens out: none of their
// callers needs them, and a lost key then still lets a user list, detach and
// relink integrations.
type IntegrationRepository struct {
	db     *gorm.DB
	cipher *crypto.Cipher
}

func NewIntegrationRepository(db *gorm.DB, cipher *crypto.Cipher) *IntegrationRepository {
	return &IntegrationRepository{db: db, cipher: cipher}
}

// IntegrationTokenAAD binds a sealed token to its row and column, so a value
// copied into another integration or into the other token column does not
// open.
func IntegrationTokenAAD(id uuid.UUID, column string) []byte {
	return []byte(id.String() + ":" + column)
}

func withoutTokens(db *gorm.DB) *gorm.DB {
	return db.Omit("access_token", "refresh_token")
}

// seal leaves an empty token empty: providers without refresh tokens store
// none, and there is nothing to protect.
func (r *IntegrationRepository) seal(id uuid.UUID, column, plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	if r.cipher == nil {
		return "", ErrNoCipher
	}
	sealed, err := r.cipher.SealString(plain, IntegrationTokenAAD(id, column))
	if err != nil {
		return "", fmt.Errorf("seal %s: %w", column, err)
	}
	return sealed, nil
}

func (r *IntegrationRepository) open(id uuid.UUID, column, stored string) (string, error) {
	if stored == "" {
		return "", nil
	}
	if r.cipher == nil {
		return "", ErrNoCipher
	}
	plain, err := r.cipher.OpenString(stored, IntegrationTokenAAD(id, column))
	if err != nil {
		return "", fmt.Errorf("open %s of integration %s: %w", column, id, err)
	}
	return plain, nil
}

func (r *IntegrationRepository) tokenColumns(it *models.OAuthIntegration) (map[string]any, error) {
	access, err := r.seal(it.ID, "access_token", it.AccessToken)
	if err != nil {
		return nil, err
	}
	refresh, err := r.seal(it.ID, "refresh_token", it.RefreshToken)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"access_token":  access,
		"refresh_token": refresh,
		"expires_at":    it.ExpiresAt,
	}, nil
}

func (r *IntegrationRepository) Create(it *models.OAuthIntegration) error {
	// The id is part of the tokens' aad, so it has to exist before the hook
	// that would otherwise assign it.
	if it.ID == uuid.Nil {
		it.ID = uuid.New()
	}
	row := *it
	var err error
	if row.AccessToken, err = r.seal(it.ID, "access_token", it.AccessToken); err != nil {
		return err
	}
	if row.RefreshToken, err = r.seal(it.ID, "refresh_token", it.RefreshToken); err != nil {
		return err
	}
	if err := r.db.Create(&row).Error; err != nil {
		return err
	}
	it.CreatedAt, it.UpdatedAt = row.CreatedAt, row.UpdatedAt
	return nil
}

func (r *IntegrationRepository) ListByOwner(ownerID uuid.UUID) ([]models.OAuthIntegration, error) {
	var out []models.OAuthIntegration
	err := r.db.Scopes(withoutTokens).Where("owner_id = ?", ownerID).Order("created_at DESC").Find(&out).Error
	return out, err
}

func (r *IntegrationRepository) GetByOwnerProviderAccount(ownerID uuid.UUID, provider, accountName string) (*models.OAuthIntegration, error) {
	var it models.OAuthIntegration
	err := r.db.Scopes(withoutTokens).Where("owner_id = ? AND provider = ? AND account_name = ?", ownerID, provider, accountName).
		First(&it).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &it, nil
}

func (r *IntegrationRepository) GetByID(id uuid.UUID) (*models.OAuthIntegration, error) {
	var it models.OAuthIntegration
	err := r.db.Scopes(withoutTokens).Where("id = ?", id).First(&it).Error
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
	cols, err := r.tokenColumns(it)
	if err != nil {
		return err
	}
	return r.db.Model(&models.OAuthIntegration{}).Where("id = ?", it.ID).Updates(cols).Error
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
		var err error
		if it.AccessToken, err = r.open(it.ID, "access_token", it.AccessToken); err != nil {
			return err
		}
		if it.RefreshToken, err = r.open(it.ID, "refresh_token", it.RefreshToken); err != nil {
			return err
		}
		changed, err := refresh(&it)
		if err != nil {
			return err
		}
		if !changed {
			return nil
		}
		cols, err := r.tokenColumns(&it)
		if err != nil {
			return err
		}
		return tx.Model(&models.OAuthIntegration{}).Where("id = ?", it.ID).Updates(cols).Error
	})
	if err != nil {
		return nil, err
	}
	return &it, nil
}
