package auth

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"regexp"
	"strings"

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

var scpLikeRe = regexp.MustCompile(`^(?:[^@:/\s]+@)?([^@:/\s]+):(.*)$`)

// parseGitRemote reads https, ssh and scp-like git URLs; an scp-like one
// (git@host:owner/repo) comes back as the equivalent ssh URL.
func parseGitRemote(raw string) (*url.URL, error) {
	if !strings.Contains(raw, "://") {
		if m := scpLikeRe.FindStringSubmatch(raw); m != nil {
			return &url.URL{Scheme: "ssh", Host: m[1], Path: "/" + strings.TrimPrefix(m[2], "/")}, nil
		}
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "" || u.Hostname() == "" {
		return nil, fmt.Errorf("git URL %q has no scheme or host", raw)
	}
	return u, nil
}

func effectivePort(u *url.URL) string {
	if p := u.Port(); p != "" {
		return p
	}
	switch strings.ToLower(u.Scheme) {
	case "http":
		return "80"
	case "https":
		return "443"
	}
	return ""
}

// authenticateOn adds credentials only when repoURL points at origin, the
// provider's own web origin; for any other host it returns the URL untouched,
// so a token is never sent to a host its provider does not run. ssh and scp
// URLs on the provider's host are rewritten to https on origin, since the
// token authenticates over https only. An https URL has to match origin's
// scheme and port as well, so the header never travels over plain http.
func authenticateOn(origin *url.URL, repoURL, username, password string) GitRemote {
	u, err := parseGitRemote(repoURL)
	if err != nil || !strings.EqualFold(u.Hostname(), origin.Hostname()) {
		return GitRemote{URL: repoURL}
	}
	switch {
	case strings.EqualFold(u.Scheme, "ssh"):
	case strings.EqualFold(u.Scheme, origin.Scheme) && effectivePort(u) == effectivePort(origin):
	default:
		return GitRemote{URL: repoURL}
	}
	return basicAuthRemote(&url.URL{Scheme: origin.Scheme, Host: origin.Host, Path: u.Path, RawPath: u.RawPath}, username, password)
}

// SameGitHost reports whether two git URLs, in any of the forms git accepts,
// name the same host.
func SameGitHost(a, b string) bool {
	ua, errA := parseGitRemote(a)
	ub, errB := parseGitRemote(b)
	return errA == nil && errB == nil && strings.EqualFold(ua.Hostname(), ub.Hostname())
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
