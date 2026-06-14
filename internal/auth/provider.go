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

type LoginProvider interface {
	Provider
	AllowRegister() bool
	LoginAuthURL(state, nonce, pkceVerifier string) string
	ExchangeLogin(ctx context.Context, code, pkceVerifier, expectedNonce string) (subject, email string, emailVerified bool, err error)
}

type IntegrationProvider interface {
	Provider
	IntegrationAuthURL(state string) string
	ExchangeIntegration(ctx context.Context, code string) (accessToken, refreshToken string, expiresAt *time.Time, err error)
	RefreshIntegration(ctx context.Context, refreshToken string) (accessToken, newRefreshToken string, expiresAt *time.Time, err error)
	AuthenticateGitURL(repoURL, accessToken string) string
}
