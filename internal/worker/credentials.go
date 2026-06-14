package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/wimwenigerkind/odoopack-registry/internal/models"
)

const refreshLeeway = 5 * time.Minute

func (q *Queue) resolveCloneURL(ctx context.Context, job SyncJob) (string, error) {
	if job.IntegrationID == nil {
		return job.GitURL, nil
	}
	it, err := q.integrationRepo.GetByID(*job.IntegrationID)
	if err != nil {
		return "", fmt.Errorf("load integration: %w", err)
	}
	provider, err := q.authRegistry.GetIntegration(it.Provider)
	if err != nil {
		return "", fmt.Errorf("integration provider %q not configured: %w", it.Provider, err)
	}

	accessToken := it.AccessToken
	if needsRefresh(it) {
		newAccess, newRefresh, exp, err := provider.RefreshIntegration(ctx, it.RefreshToken)
		if err != nil {
			return "", fmt.Errorf("refresh integration %s: %w", it.Provider, err)
		}
		it.AccessToken = newAccess
		if newRefresh != "" {
			it.RefreshToken = newRefresh
		}
		it.ExpiresAt = exp
		if err := q.integrationRepo.UpdateTokens(it); err != nil {
			return "", fmt.Errorf("persist refreshed tokens: %w", err)
		}
		accessToken = newAccess
	}

	return provider.AuthenticateGitURL(job.GitURL, accessToken), nil
}

func needsRefresh(it *models.OAuthIntegration) bool {
	if it.ExpiresAt == nil {
		return false
	}
	return it.ExpiresAt.Before(time.Now().Add(refreshLeeway))
}
