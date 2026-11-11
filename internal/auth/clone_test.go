package auth

import (
	"encoding/base64"
	"net/url"
	"slices"
	"strings"
	"testing"
)

func TestAuthenticateGitRemote(t *testing.T) {
	forgejo, err := NewForgejoProvider("forgejo", ForgejoConfig{BaseURL: "https://code.example.com"}, "")
	if err != nil {
		t.Fatal(err)
	}
	github := NewGitHubProvider("github", GitHubConfig{}, "")
	bitbucket := NewBitbucketProvider("bitbucket", BitbucketConfig{}, "")

	const token = "s3cret-token"
	cases := []struct {
		name     string
		provider IntegrationProvider
		repoURL  string
		wantURL  string
		wantAuth string
	}{
		{"github https", github, "https://github.com/o/r.git", "https://github.com/o/r.git", "oauth2:" + token},
		{"github scp", github, "git@github.com:o/r.git", "https://github.com/o/r.git", "oauth2:" + token},
		{"bitbucket https with user", bitbucket, "https://someone@bitbucket.org/o/r.git", "https://bitbucket.org/o/r.git", "x-token-auth:" + token},
		{"bitbucket scp", bitbucket, "git@bitbucket.org:o/r.git", "https://bitbucket.org/o/r.git", "x-token-auth:" + token},
		{"bitbucket ssh", bitbucket, "ssh://git@bitbucket.org/o/r.git", "https://bitbucket.org/o/r.git", "x-token-auth:" + token},
		{"forgejo https", forgejo, "https://code.example.com/o/r.git", "https://code.example.com/o/r.git", token + ":"},
		{"forgejo scp", forgejo, "git@code.example.com:o/r.git", "https://code.example.com/o/r.git", token + ":"},
		{"forgejo ssh", forgejo, "ssh://git@code.example.com/o/r.git", "https://code.example.com/o/r.git", token + ":"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			remote := tc.provider.AuthenticateGitRemote(tc.repoURL, token)
			if remote.URL != tc.wantURL {
				t.Errorf("url %q, want %q", remote.URL, tc.wantURL)
			}
			if u, err := url.Parse(remote.URL); err != nil || u.User != nil {
				t.Errorf("url %q carries userinfo", remote.URL)
			}
			if strings.Contains(remote.URL, token) {
				t.Errorf("url %q contains the token", remote.URL)
			}
			u, _ := url.Parse(tc.wantURL)
			wantEnv := []string{
				"GIT_CONFIG_COUNT=1",
				"GIT_CONFIG_KEY_0=http." + u.Scheme + "://" + u.Host + "/.extraHeader",
				"GIT_CONFIG_VALUE_0=Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte(tc.wantAuth)),
			}
			if !slices.Equal(remote.Env, wantEnv) {
				t.Errorf("env %q, want %q", remote.Env, wantEnv)
			}
		})
	}
}
