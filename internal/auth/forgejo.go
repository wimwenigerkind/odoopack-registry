package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

const ProviderTypeForgejo ProviderType = "forgejo"

type ForgejoConfig struct {
	AllowLogin          bool
	AllowRegister       bool
	AllowGitIntegration bool
	BaseURL             string
	ClientID            string
	ClientSecret        string
}

type ForgejoProvider struct {
	name        string
	cfg         ForgejoConfig
	baseURL     string
	apiBase     string
	host        string
	loginOAuth  *oauth2.Config
	integOAuth  *oauth2.Config
	callbackURL string
}

func NewForgejoProvider(name string, cfg ForgejoConfig, callbackURL string) (*ForgejoProvider, error) {
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		return nil, fmt.Errorf("base_url is required")
	}
	u, err := url.Parse(base)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("invalid base_url %q", cfg.BaseURL)
	}

	endpoint := oauth2.Endpoint{
		AuthURL:  base + "/login/oauth/authorize",
		TokenURL: base + "/login/oauth/access_token",
	}

	return &ForgejoProvider{
		name:        name,
		cfg:         cfg,
		baseURL:     base,
		apiBase:     base + "/api/v1",
		host:        u.Host,
		callbackURL: callbackURL,
		loginOAuth: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Endpoint:     endpoint,
			RedirectURL:  callbackURL,
			Scopes:       []string{"read:user"},
		},
		integOAuth: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Endpoint:     endpoint,
			RedirectURL:  callbackURL,
			Scopes:       []string{"read:user", "read:repository"},
		},
	}, nil
}

func (p *ForgejoProvider) Name() string              { return p.name }
func (p *ForgejoProvider) Type() ProviderType        { return ProviderTypeForgejo }
func (p *ForgejoProvider) AllowLogin() bool          { return p.cfg.AllowLogin }
func (p *ForgejoProvider) AllowRegister() bool       { return p.cfg.AllowRegister }
func (p *ForgejoProvider) AllowGitIntegration() bool { return p.cfg.AllowGitIntegration }

func (p *ForgejoProvider) LoginAuthURL(state, _, _ string) string {
	return p.loginOAuth.AuthCodeURL(state)
}

func (p *ForgejoProvider) ExchangeLogin(ctx context.Context, code, _, _ string) (LoginResult, error) {
	token, err := p.loginOAuth.Exchange(ctx, code)
	if err != nil {
		return LoginResult{}, fmt.Errorf("forgejo token exchange: %w", err)
	}
	user, err := p.fetchUser(ctx, token.AccessToken)
	if err != nil {
		return LoginResult{}, err
	}
	primary, verified, err := p.fetchPrimaryEmail(ctx, token.AccessToken)
	if err != nil {
		return LoginResult{}, err
	}
	if primary == "" {
		primary = user.Email
	}
	return LoginResult{Subject: fmt.Sprintf("%d", user.ID), Email: primary, EmailVerified: verified}, nil
}

func (p *ForgejoProvider) IntegrationAuthURL(state string) string {
	return p.integOAuth.AuthCodeURL(state)
}

func (p *ForgejoProvider) ExchangeIntegration(ctx context.Context, code string) (accessToken, refreshToken string, expiresAt *time.Time, err error) {
	token, err := p.integOAuth.Exchange(ctx, code)
	if err != nil {
		return "", "", nil, fmt.Errorf("forgejo integration exchange: %w", err)
	}
	return token.AccessToken, token.RefreshToken, tokenExpiry(token), nil
}

func (p *ForgejoProvider) RefreshIntegration(ctx context.Context, refresh string) (accessToken, newRefresh string, expiresAt *time.Time, err error) {
	token, err := p.integOAuth.TokenSource(ctx, &oauth2.Token{RefreshToken: refresh}).Token()
	if err != nil {
		return "", "", nil, fmt.Errorf("forgejo integration refresh: %w", err)
	}
	return token.AccessToken, token.RefreshToken, tokenExpiry(token), nil
}

func (p *ForgejoProvider) FetchAccountName(ctx context.Context, accessToken string) (string, error) {
	user, err := p.fetchUser(ctx, accessToken)
	if err != nil {
		return "", err
	}
	return user.Login, nil
}

func (p *ForgejoProvider) AuthenticateGitURL(repoURL, accessToken string) string {
	sshPrefix := "git@" + p.host + ":"
	if after, ok := strings.CutPrefix(repoURL, sshPrefix); ok {
		path := after
		return fmt.Sprintf("https://%s@%s/%s", url.QueryEscape(accessToken), p.host, path)
	}
	sshURLPrefix := "ssh://git@" + p.host + "/"
	if after, ok := strings.CutPrefix(repoURL, sshURLPrefix); ok {
		path := after
		return fmt.Sprintf("https://%s@%s/%s", url.QueryEscape(accessToken), p.host, path)
	}
	u, err := url.Parse(repoURL)
	if err != nil || u.Scheme == "" {
		return repoURL
	}
	u.User = url.User(accessToken)
	return u.String()
}

type forgejoUser struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Email string `json:"email"`
}

func (p *ForgejoProvider) fetchUser(ctx context.Context, token string) (*forgejoUser, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, p.apiBase+"/user", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("forgejo user: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("forgejo user: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var u forgejoUser
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, fmt.Errorf("forgejo user decode: %w", err)
	}
	if u.ID == 0 {
		return nil, fmt.Errorf("forgejo user: missing id")
	}
	return &u, nil
}

type forgejoEmail struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

func (p *ForgejoProvider) fetchPrimaryEmail(ctx context.Context, token string) (string, bool, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, p.apiBase+"/user/emails", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("forgejo emails: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("forgejo emails: %s", resp.Status)
	}
	var emails []forgejoEmail
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", false, fmt.Errorf("forgejo emails decode: %w", err)
	}
	for _, e := range emails {
		if e.Primary {
			return e.Email, e.Verified, nil
		}
	}
	if len(emails) > 0 {
		return emails[0].Email, emails[0].Verified, nil
	}
	return "", false, nil
}
