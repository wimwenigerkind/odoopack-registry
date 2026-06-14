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
	githubendpoint "golang.org/x/oauth2/github"
)

const ProviderTypeGitHub ProviderType = "github"

type GitHubConfig struct {
	AllowLogin          bool
	AllowRegister       bool
	AllowGitIntegration bool
	ClientID            string
	ClientSecret        string
}

type GitHubProvider struct {
	name        string
	cfg         GitHubConfig
	loginOAuth  *oauth2.Config
	integOAuth  *oauth2.Config
	callbackURL string
}

func NewGitHubProvider(name string, cfg GitHubConfig, callbackURL string) *GitHubProvider {
	return &GitHubProvider{
		name:        name,
		cfg:         cfg,
		callbackURL: callbackURL,
		loginOAuth: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Endpoint:     githubendpoint.Endpoint,
			RedirectURL:  callbackURL,
			Scopes:       []string{"read:user", "user:email"},
		},
		integOAuth: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Endpoint:     githubendpoint.Endpoint,
			RedirectURL:  callbackURL,
			Scopes:       []string{"repo"},
		},
	}
}

func (p *GitHubProvider) Name() string              { return p.name }
func (p *GitHubProvider) Type() ProviderType        { return ProviderTypeGitHub }
func (p *GitHubProvider) AllowLogin() bool          { return p.cfg.AllowLogin }
func (p *GitHubProvider) AllowRegister() bool       { return p.cfg.AllowRegister }
func (p *GitHubProvider) AllowGitIntegration() bool { return p.cfg.AllowGitIntegration }

func (p *GitHubProvider) LoginAuthURL(state, _, _ string) string {
	return p.loginOAuth.AuthCodeURL(state)
}

func (p *GitHubProvider) ExchangeLogin(ctx context.Context, code, _, _ string) (subject, email string, emailVerified bool, err error) {
	token, err := p.loginOAuth.Exchange(ctx, code)
	if err != nil {
		return "", "", false, fmt.Errorf("github token exchange: %w", err)
	}
	user, err := fetchGitHubUser(ctx, token.AccessToken)
	if err != nil {
		return "", "", false, err
	}
	primary, verified, err := fetchGitHubPrimaryEmail(ctx, token.AccessToken)
	if err != nil {
		return "", "", false, err
	}
	return fmt.Sprintf("%d", user.ID), primary, verified, nil
}

func (p *GitHubProvider) IntegrationAuthURL(state string) string {
	return p.integOAuth.AuthCodeURL(state)
}

func (p *GitHubProvider) ExchangeIntegration(ctx context.Context, code string) (accessToken, refreshToken string, expiresAt *time.Time, err error) {
	token, err := p.integOAuth.Exchange(ctx, code)
	if err != nil {
		return "", "", nil, fmt.Errorf("github integration exchange: %w", err)
	}
	return token.AccessToken, token.RefreshToken, tokenExpiry(token), nil
}

func (p *GitHubProvider) RefreshIntegration(ctx context.Context, refresh string) (accessToken, newRefresh string, expiresAt *time.Time, err error) {
	token, err := p.integOAuth.TokenSource(ctx, &oauth2.Token{RefreshToken: refresh}).Token()
	if err != nil {
		return "", "", nil, fmt.Errorf("github integration refresh: %w", err)
	}
	return token.AccessToken, token.RefreshToken, tokenExpiry(token), nil
}

func (p *GitHubProvider) AuthenticateGitURL(repoURL, accessToken string) string {
	u, err := url.Parse(repoURL)
	if err != nil || u.Scheme == "" {
		return repoURL
	}
	u.User = url.UserPassword("oauth2", accessToken)
	return u.String()
}

func tokenExpiry(token *oauth2.Token) *time.Time {
	if token.Expiry.IsZero() {
		return nil
	}
	t := token.Expiry
	return &t
}

type ghUser struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}

func fetchGitHubUser(ctx context.Context, token string) (*ghUser, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github user: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github user: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var u ghUser
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, fmt.Errorf("github user decode: %w", err)
	}
	if u.ID == 0 {
		return nil, fmt.Errorf("github user: missing id")
	}
	return &u, nil
}

type ghEmail struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

func fetchGitHubPrimaryEmail(ctx context.Context, token string) (string, bool, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/emails", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("github emails: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("github emails: %s", resp.Status)
	}
	var emails []ghEmail
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", false, fmt.Errorf("github emails decode: %w", err)
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
