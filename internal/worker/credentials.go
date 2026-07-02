package worker

import (
	"context"

	"github.com/wimwenigerkind/odoopack-registry/internal/auth"
)

func (q *Queue) resolveCloneURL(ctx context.Context, job SyncJob) (string, error) {
	return auth.ResolveCloneURL(ctx, q.authRegistry, q.integrationRepo, job.GitURL, job.IntegrationID)
}
