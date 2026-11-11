package worker

import (
	"context"

	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/auth"
)

func (c *Consumer) resolveGitRemote(ctx context.Context, gitURL string, integrationID *uuid.UUID) (auth.GitRemote, error) {
	return auth.ResolveGitRemote(ctx, c.authRegistry, c.integrationRepo, gitURL, integrationID)
}
