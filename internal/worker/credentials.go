package worker

import (
	"context"

	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/auth"
)

func (c *Consumer) resolveCloneURL(ctx context.Context, gitURL string, integrationID *uuid.UUID) (string, error) {
	return auth.ResolveCloneURL(ctx, c.authRegistry, c.integrationRepo, gitURL, integrationID)
}
