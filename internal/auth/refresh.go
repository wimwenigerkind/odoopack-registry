package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
)

const RefreshLeeway = 5 * time.Minute

type TokenStore interface {
	RefreshTokensWithLock(ctx context.Context, id uuid.UUID, refresh func(*models.OAuthIntegration) (bool, error)) (*models.OAuthIntegration, error)
}

func EnsureFreshToken(ctx context.Context, registry *Registry, store TokenStore, it *models.OAuthIntegration) (string, error) {
	var access string
	_, err := store.RefreshTokensWithLock(ctx, it.ID, func(cur *models.OAuthIntegration) (bool, error) {
		if cur.ExpiresAt == nil || cur.ExpiresAt.After(time.Now().Add(RefreshLeeway)) {
			access = cur.AccessToken
			return false, nil
		}
		provider, err := registry.GetIntegration(cur.Provider)
		if err != nil {
			return false, err
		}
		newAccess, newRefresh, exp, err := provider.RefreshIntegration(ctx, cur.RefreshToken)
		if err != nil {
			return false, err
		}
		cur.AccessToken = newAccess
		if newRefresh != "" {
			cur.RefreshToken = newRefresh
		}
		cur.ExpiresAt = exp
		access = newAccess
		return true, nil
	})
	if err != nil {
		return "", err
	}
	return access, nil
}
