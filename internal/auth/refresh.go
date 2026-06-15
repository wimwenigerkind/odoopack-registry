package auth

import (
	"context"
	"time"

	"github.com/wimwenigerkind/odoopack-registry/internal/models"
)

const RefreshLeeway = 5 * time.Minute

type TokenUpdater interface {
	UpdateTokens(it *models.OAuthIntegration) error
}

func EnsureFreshToken(ctx context.Context, registry *Registry, updater TokenUpdater, it *models.OAuthIntegration) (string, error) {
	if it.ExpiresAt == nil || it.ExpiresAt.After(time.Now().Add(RefreshLeeway)) {
		return it.AccessToken, nil
	}
	provider, err := registry.GetIntegration(it.Provider)
	if err != nil {
		return "", err
	}
	newAccess, newRefresh, exp, err := provider.RefreshIntegration(ctx, it.RefreshToken)
	if err != nil {
		return "", err
	}
	it.AccessToken = newAccess
	if newRefresh != "" {
		it.RefreshToken = newRefresh
	}
	it.ExpiresAt = exp
	if err := updater.UpdateTokens(it); err != nil {
		return "", err
	}
	return newAccess, nil
}
