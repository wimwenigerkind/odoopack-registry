package auth

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type OIDCConfig struct {
	Type          string `mapstructure:"type"`
	AllowLogin    bool   `mapstructure:"allow_login"`
	AllowRegister bool   `mapstructure:"allow_register"`
	IssuerURL     string `mapstructure:"issuer_url"`
	ClientID      string `mapstructure:"client_id"`
	ClientSecret  string `mapstructure:"client_secret"`
}

type OidcProvider struct {
	name     string
	cfg      OIDCConfig
	oauth2   *oauth2.Config
	verifier *oidc.IDTokenVerifier
}

func NewOIDCProvider(ctx context.Context, name string, cfg OIDCConfig, redirectURL string) (*OidcProvider, error) {
	provider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery: %w", err)
	}
	return &OidcProvider{
		name: name,
		cfg:  cfg,
		oauth2: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Endpoint:     provider.Endpoint(),
			RedirectURL:  redirectURL,
			Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
		},
		verifier: provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}),
	}, nil
}

func (p *OidcProvider) Name() string              { return p.name }
func (p *OidcProvider) Type() ProviderType        { return OIDC }
func (p *OidcProvider) AllowLogin() bool          { return p.cfg.AllowLogin }
func (p *OidcProvider) AllowRegister() bool       { return p.cfg.AllowRegister }
func (p *OidcProvider) AllowGitIntegration() bool { return false }

func (p *OidcProvider) LoginAuthURL(state, nonce, pkceVerifier string) string {
	return p.oauth2.AuthCodeURL(
		state,
		oidc.Nonce(nonce),
		oauth2.S256ChallengeOption(pkceVerifier),
	)
}

func (p *OidcProvider) ExchangeLogin(ctx context.Context, code, pkceVerifier, expectedNonce string) (subject, email string, emailVerified bool, err error) {
	token, err := p.oauth2.Exchange(ctx, code, oauth2.VerifierOption(pkceVerifier))
	if err != nil {
		return "", "", false, fmt.Errorf("token exchange: %w", err)
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return "", "", false, fmt.Errorf("no id_token in token response")
	}
	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return "", "", false, fmt.Errorf("id_token verify: %w", err)
	}
	if idToken.Nonce != expectedNonce {
		return "", "", false, fmt.Errorf("nonce mismatch")
	}
	if idToken.Subject == "" {
		return "", "", false, fmt.Errorf("id_token missing subject")
	}
	var claims struct {
		Email             string `json:"email"`
		EmailVerified     bool   `json:"email_verified"`
		Name              string `json:"name"`
		PreferredUsername string `json:"preferred_username"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return "", "", false, fmt.Errorf("parse claims: %w", err)
	}
	return idToken.Subject, claims.Email, claims.EmailVerified, nil
}
