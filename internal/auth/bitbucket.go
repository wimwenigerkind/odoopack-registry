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

const ProviderTypeBitbucket ProviderType = "bitbucket"

var bitbucketEndpoint = oauth2.Endpoint{
	AuthURL:   "https://bitbucket.org/site/oauth2/authorize",
	TokenURL:  "https://bitbucket.org/site/oauth2/access_token",
	AuthStyle: oauth2.AuthStyleInHeader,
}

type BitbucketConfig struct {
	AllowLogin          bool
	AllowRegister       bool
	AllowGitIntegration bool
	ClientID            string
	ClientSecret        string
}

type BitbucketProvider struct {
	name        string
	cfg         BitbucketConfig
	oauth2      *oauth2.Config
	callbackURL string
}

func NewBitbucketProvider(name string, cfg BitbucketConfig, callbackURL string) *BitbucketProvider {
	return &BitbucketProvider{
		name:        name,
		cfg:         cfg,
		callbackURL: callbackURL,
		oauth2: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Endpoint:     bitbucketEndpoint,
			RedirectURL:  callbackURL,
		},
	}
}

func (p *BitbucketProvider) Name() string              { return p.name }
func (p *BitbucketProvider) Type() ProviderType        { return ProviderTypeBitbucket }
func (p *BitbucketProvider) AllowLogin() bool          { return p.cfg.AllowLogin }
func (p *BitbucketProvider) AllowRegister() bool       { return p.cfg.AllowRegister }
func (p *BitbucketProvider) AllowGitIntegration() bool { return p.cfg.AllowGitIntegration }

func (p *BitbucketProvider) LoginAuthURL(state, _, _ string) string {
	return p.oauth2.AuthCodeURL(state)
}

func (p *BitbucketProvider) ExchangeLogin(ctx context.Context, code, _, _ string) (LoginResult, error) {
	token, err := p.oauth2.Exchange(ctx, code)
	if err != nil {
		return LoginResult{}, fmt.Errorf("bitbucket token exchange: %w", err)
	}
	user, err := fetchBitbucketUser(ctx, token.AccessToken)
	if err != nil {
		return LoginResult{}, err
	}
	primary, verified, err := fetchBitbucketPrimaryEmail(ctx, token.AccessToken)
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{Subject: user.UUID, Email: primary, EmailVerified: verified}, nil
}

func (p *BitbucketProvider) IntegrationAuthURL(state string) string {
	return p.oauth2.AuthCodeURL(state)
}

func (p *BitbucketProvider) ExchangeIntegration(ctx context.Context, code string) (accessToken, refreshToken string, expiresAt *time.Time, err error) {
	token, err := p.oauth2.Exchange(ctx, code)
	if err != nil {
		return "", "", nil, fmt.Errorf("bitbucket integration exchange: %w", err)
	}
	return token.AccessToken, token.RefreshToken, tokenExpiry(token), nil
}

func (p *BitbucketProvider) RefreshIntegration(ctx context.Context, refresh string) (accessToken, newRefresh string, expiresAt *time.Time, err error) {
	token, err := p.oauth2.TokenSource(ctx, &oauth2.Token{RefreshToken: refresh}).Token()
	if err != nil {
		return "", "", nil, fmt.Errorf("bitbucket integration refresh: %w", err)
	}
	return token.AccessToken, token.RefreshToken, tokenExpiry(token), nil
}

func (p *BitbucketProvider) FetchAccountName(ctx context.Context, accessToken string) (string, error) {
	user, err := fetchBitbucketUser(ctx, accessToken)
	if err != nil {
		return "", err
	}
	if user.Username != "" {
		return user.Username, nil
	}
	if user.Nickname != "" {
		return user.Nickname, nil
	}
	return user.DisplayName, nil
}

func (p *BitbucketProvider) AuthenticateGitURL(repoURL, accessToken string) string {
	if after, ok := strings.CutPrefix(repoURL, "git@bitbucket.org:"); ok {
		path := after
		return fmt.Sprintf("https://x-token-auth:%s@bitbucket.org/%s", accessToken, path)
	}
	if after, ok := strings.CutPrefix(repoURL, "ssh://git@bitbucket.org/"); ok {
		path := after
		return fmt.Sprintf("https://x-token-auth:%s@bitbucket.org/%s", accessToken, path)
	}
	u, err := url.Parse(repoURL)
	if err != nil || u.Scheme == "" {
		return repoURL
	}
	u.User = url.UserPassword("x-token-auth", accessToken)
	return u.String()
}

type bbUser struct {
	UUID        string `json:"uuid"`
	Username    string `json:"username"`
	Nickname    string `json:"nickname"`
	DisplayName string `json:"display_name"`
}

func fetchBitbucketUser(ctx context.Context, token string) (*bbUser, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.bitbucket.org/2.0/user", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bitbucket user: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bitbucket user: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var u bbUser
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, fmt.Errorf("bitbucket user decode: %w", err)
	}
	if u.UUID == "" {
		return nil, fmt.Errorf("bitbucket user: missing uuid")
	}
	return &u, nil
}

type bbEmail struct {
	Email      string `json:"email"`
	IsPrimary  bool   `json:"is_primary"`
	IsVerified bool   `json:"is_confirmed"`
}

type bbEmailsResponse struct {
	Values []bbEmail `json:"values"`
}

func fetchBitbucketPrimaryEmail(ctx context.Context, token string) (string, bool, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.bitbucket.org/2.0/user/emails", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("bitbucket emails: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("bitbucket emails: %s", resp.Status)
	}
	var resBody bbEmailsResponse
	if err := json.NewDecoder(resp.Body).Decode(&resBody); err != nil {
		return "", false, fmt.Errorf("bitbucket emails decode: %w", err)
	}
	for _, e := range resBody.Values {
		if e.IsPrimary {
			return e.Email, e.IsVerified, nil
		}
	}
	if len(resBody.Values) > 0 {
		return resBody.Values[0].Email, resBody.Values[0].IsVerified, nil
	}
	return "", false, nil
}
