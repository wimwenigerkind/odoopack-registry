package auth

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
)

type IntegrationStore interface {
	GetByID(id uuid.UUID) (*models.OAuthIntegration, error)
	RefreshTokensWithLock(ctx context.Context, id uuid.UUID, refresh func(*models.OAuthIntegration) (bool, error)) (*models.OAuthIntegration, error)
}

func ResolveCloneURL(ctx context.Context, registry *Registry, store IntegrationStore, gitURL string, integrationID *uuid.UUID) (string, error) {
	if integrationID == nil {
		return gitURL, nil
	}
	it, err := store.GetByID(*integrationID)
	if err != nil {
		return "", fmt.Errorf("load integration: %w", err)
	}
	provider, err := registry.GetIntegration(it.Provider)
	if err != nil {
		return "", fmt.Errorf("integration provider %q not configured: %w", it.Provider, err)
	}
	accessToken, err := EnsureFreshToken(ctx, registry, store, it)
	if err != nil {
		return "", fmt.Errorf("ensure fresh token: %w", err)
	}
	return provider.AuthenticateGitURL(gitURL, accessToken), nil
}
