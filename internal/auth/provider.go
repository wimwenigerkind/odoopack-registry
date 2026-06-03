package auth

import "context"

type ProviderType string

const (
	OIDC ProviderType = "oidc"
)

type Provider interface {
	Name() string
	Type() ProviderType
	AllowLogin() bool
	AllowRegister() bool
	AuthURL(state, nonce, pkceVerifier string) string
	Exchange(ctx context.Context, code, pkceVerifier, expectedNonce string) (subject, email string, emailVerified bool, err error)
}
