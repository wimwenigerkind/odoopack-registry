package worker

import (
	"context"
	"fmt"

	"github.com/wimwenigerkind/odoopack-registry/internal/auth"
)

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
	accessToken, err := auth.EnsureFreshToken(ctx, q.authRegistry, q.integrationRepo, it)
	if err != nil {
		return "", fmt.Errorf("ensure fresh token: %w", err)
	}
	return provider.AuthenticateGitURL(job.GitURL, accessToken), nil
}
