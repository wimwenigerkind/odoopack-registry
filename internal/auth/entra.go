package auth

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

const ProviderTypeEntra ProviderType = "entra"

type EntraConfig struct {
	AllowLogin    bool
	AllowRegister bool
	TenantID      string
	ClientID      string
	ClientSecret  string
}

type EntraProvider struct {
	name     string
	cfg      EntraConfig
	oauth2   *oauth2.Config
	verifier *oidc.IDTokenVerifier
}

func NewEntraProvider(ctx context.Context, name string, cfg EntraConfig, redirectURL string) (*EntraProvider, error) {
	tenant := cfg.TenantID
	if tenant == "" {
		tenant = "common"
	}
	issuer := "https://login.microsoftonline.com/" + tenant + "/v2.0"
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("entra discovery: %w", err)
	}
	return &EntraProvider{
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

func (p *EntraProvider) Name() string              { return p.name }
func (p *EntraProvider) Type() ProviderType        { return ProviderTypeEntra }
func (p *EntraProvider) AllowLogin() bool          { return p.cfg.AllowLogin }
func (p *EntraProvider) AllowRegister() bool       { return p.cfg.AllowRegister }
func (p *EntraProvider) AllowGitIntegration() bool { return false }

func (p *EntraProvider) LoginAuthURL(state, nonce, pkceVerifier string) string {
	return p.oauth2.AuthCodeURL(
		state,
		oidc.Nonce(nonce),
		oauth2.S256ChallengeOption(pkceVerifier),
	)
}

func (p *EntraProvider) ExchangeLogin(ctx context.Context, code, pkceVerifier, expectedNonce string) (LoginResult, error) {
	token, err := p.oauth2.Exchange(ctx, code, oauth2.VerifierOption(pkceVerifier))
	if err != nil {
		return LoginResult{}, fmt.Errorf("token exchange: %w", err)
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return LoginResult{}, fmt.Errorf("no id_token in token response")
	}
	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return LoginResult{}, fmt.Errorf("id_token verify: %w", err)
	}
	if idToken.Nonce != expectedNonce {
		return LoginResult{}, fmt.Errorf("nonce mismatch")
	}
	if idToken.Subject == "" {
		return LoginResult{}, fmt.Errorf("id_token missing subject")
	}
	var claims struct {
		Email             string `json:"email"`
		PreferredUsername string `json:"preferred_username"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return LoginResult{}, fmt.Errorf("parse claims: %w", err)
	}
	resolvedEmail := claims.Email
	if resolvedEmail == "" {
		resolvedEmail = claims.PreferredUsername
	}
	return LoginResult{Subject: idToken.Subject, Email: resolvedEmail, EmailVerified: true}, nil
}
