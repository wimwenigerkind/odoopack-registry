package auth

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type OIDCConfig struct {
	Type                string              `mapstructure:"type"`
	AllowLogin          bool                `mapstructure:"allow_login"`
	AllowRegister       bool                `mapstructure:"allow_register"`
	IssuerURL           string              `mapstructure:"issuer_url"`
	ClientID            string              `mapstructure:"client_id"`
	ClientSecret        string              `mapstructure:"client_secret"`
	GroupClaimName      string              `mapstructure:"group_claim_name"`
	AdminGroup          string              `mapstructure:"admin_group"`
	GroupTeamMapRemoval bool                `mapstructure:"group_team_map_removal"`
	GroupTeamMap        map[string][]string `mapstructure:"group_team_map"`
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

func (p *OidcProvider) GroupTeamMap() map[string][]string { return p.cfg.GroupTeamMap }
func (p *OidcProvider) AdminGroup() string                { return p.cfg.AdminGroup }
func (p *OidcProvider) GroupTeamMapRemoval() bool         { return p.cfg.GroupTeamMapRemoval }

func (p *OidcProvider) LoginAuthURL(state, nonce, pkceVerifier string) string {
	return p.oauth2.AuthCodeURL(
		state,
		oidc.Nonce(nonce),
		oauth2.S256ChallengeOption(pkceVerifier),
	)
}

func (p *OidcProvider) ExchangeLogin(ctx context.Context, code, pkceVerifier, expectedNonce string) (LoginResult, error) {
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
		EmailVerified     bool   `json:"email_verified"`
		Name              string `json:"name"`
		PreferredUsername string `json:"preferred_username"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return LoginResult{}, fmt.Errorf("parse claims: %w", err)
	}

	claimName := p.cfg.GroupClaimName
	if claimName == "" {
		claimName = "groups"
	}
	var allClaims map[string]any
	_ = idToken.Claims(&allClaims)

	return LoginResult{
		Subject:       idToken.Subject,
		Email:         claims.Email,
		EmailVerified: claims.EmailVerified,
		Groups:        stringSliceClaim(allClaims[claimName]),
	}, nil
}

func stringSliceClaim(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}
