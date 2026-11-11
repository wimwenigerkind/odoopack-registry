package auth

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"

	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
)

type IntegrationStore interface {
	GetByID(id uuid.UUID) (*models.OAuthIntegration, error)
	RefreshTokensWithLock(ctx context.Context, id uuid.UUID, refresh func(*models.OAuthIntegration) (bool, error)) (*models.OAuthIntegration, error)
}

// GitRemote is a clone URL without credentials plus the environment git
// needs to authenticate against it. The token travels as a config value in
// the environment, so it shows up neither in the URL, in git's error output,
// nor in the process arguments.
type GitRemote struct {
	URL string
	Env []string
}

// basicAuthRemote scopes the header to the remote's origin, so git does not
// send it on to another host.
func basicAuthRemote(u *url.URL, username, password string) GitRemote {
	u.User = nil
	origin := u.Scheme + "://" + u.Host + "/"
	credentials := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	return GitRemote{
		URL: u.String(),
		Env: []string{
			"GIT_CONFIG_COUNT=1",
			"GIT_CONFIG_KEY_0=http." + origin + ".extraHeader",
			"GIT_CONFIG_VALUE_0=Authorization: Basic " + credentials,
		},
	}
}

func ResolveGitRemote(ctx context.Context, registry *Registry, store IntegrationStore, gitURL string, integrationID *uuid.UUID) (GitRemote, error) {
	if integrationID == nil {
		return GitRemote{URL: gitURL}, nil
	}
	it, err := store.GetByID(*integrationID)
	if err != nil {
		return GitRemote{}, fmt.Errorf("load integration: %w", err)
	}
	provider, err := registry.GetIntegration(it.Provider)
	if err != nil {
		return GitRemote{}, fmt.Errorf("integration provider %q not configured: %w", it.Provider, err)
	}
	accessToken, err := EnsureFreshToken(ctx, registry, store, it)
	if err != nil {
		return GitRemote{}, fmt.Errorf("ensure fresh token: %w", err)
	}
	return provider.AuthenticateGitRemote(gitURL, accessToken), nil
}
