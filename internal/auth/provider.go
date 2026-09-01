package auth

import (
	"context"
	"time"
)

type ProviderType string

const (
	OIDC ProviderType = "oidc"
)

type Provider interface {
	Name() string
	Type() ProviderType
	AllowLogin() bool
	AllowGitIntegration() bool
}

type LoginResult struct {
	Subject       string
	Email         string
	EmailVerified bool
	Groups        []string
}

type LoginProvider interface {
	Provider
	AllowRegister() bool
	LoginAuthURL(state, nonce, pkceVerifier string) string
	ExchangeLogin(ctx context.Context, code, pkceVerifier, expectedNonce string) (LoginResult, error)
}

type GroupMapper interface {
	GroupTeamMap() map[string][]string
	AdminGroup() string
	GroupTeamMapRemoval() bool
}

type IntegrationProvider interface {
	Provider
	IntegrationAuthURL(state string) string
	ExchangeIntegration(ctx context.Context, code string) (accessToken, refreshToken string, expiresAt *time.Time, err error)
	RefreshIntegration(ctx context.Context, refreshToken string) (accessToken, newRefreshToken string, expiresAt *time.Time, err error)
	FetchAccountName(ctx context.Context, accessToken string) (string, error)
	AuthenticateGitURL(repoURL, accessToken string) string
}
