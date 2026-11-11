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

func TestAuthenticateGitRemoteBindsHost(t *testing.T) {
	forgejo, err := NewForgejoProvider("forgejo", ForgejoConfig{BaseURL: "https://Code.Example.com:3000/forgejo"}, "")
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
	}{
		{"github host", github, "https://github.com/o/r.git", "https://github.com/o/r.git"},
		{"github host in upper case", github, "https://GitHub.COM/o/r.git", "https://github.com/o/r.git"},
		{"github default port", github, "https://github.com:443/o/r.git", "https://github.com/o/r.git"},
		{"github scp in upper case", github, "git@GITHUB.com:o/r.git", "https://github.com/o/r.git"},
		{"github scp without user", github, "github.com:o/r.git", "https://github.com/o/r.git"},
		{"github ssh with port", github, "ssh://git@github.com:22/o/r.git", "https://github.com/o/r.git"},
		{"github on acme", github, "https://git.acme.example/o/r.git", ""},
		{"github on look-alike host", github, "https://github.com.acme.example/o/r.git", ""},
		{"github on subdomain", github, "https://gist.github.com/o/r.git", ""},
		{"github over http", github, "http://github.com/o/r.git", ""},
		{"github on other port", github, "https://github.com:8443/o/r.git", ""},
		{"github scp on acme", github, "git@git.acme.example:o/r.git", ""},
		{"github git protocol", github, "git://github.com/o/r.git", ""},
		{"github userinfo host trick", github, "https://github.com@git.acme.example/o/r.git", ""},
		{"bitbucket host", bitbucket, "https://bitbucket.org/o/r.git", "https://bitbucket.org/o/r.git"},
		{"bitbucket on github", bitbucket, "https://github.com/o/r.git", ""},
		{"bitbucket scp on acme", bitbucket, "git@git.acme.example:o/r.git", ""},
		{"forgejo base_url host", forgejo, "https://code.example.com:3000/o/r.git", "https://Code.Example.com:3000/o/r.git"},
		{"forgejo scp", forgejo, "git@code.example.com:o/r.git", "https://Code.Example.com:3000/o/r.git"},
		{"forgejo without its port", forgejo, "https://code.example.com/o/r.git", ""},
		{"forgejo on github", forgejo, "https://github.com/o/r.git", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			remote := tc.provider.AuthenticateGitRemote(tc.repoURL, token)
			if tc.wantURL == "" {
				if remote.URL != tc.repoURL || remote.Env != nil {
					t.Errorf("got %+v, want %q without credentials", remote, tc.repoURL)
				}
				return
			}
			if remote.URL != tc.wantURL {
				t.Errorf("url %q, want %q", remote.URL, tc.wantURL)
			}
			if len(remote.Env) == 0 {
				t.Error("no credentials")
			}
		})
	}
}

func TestSameGitHost(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"https://github.com/o/r.git", "https://github.com/o/other.git", true},
		{"https://github.com/o/r.git", "git@GitHub.com:o/r.git", true},
		{"ssh://git@github.com:22/o/r.git", "https://github.com:443/o/r.git", true},
		{"https://github.com/o/r.git", "https://git.acme.example/o/r.git", false},
		{"https://github.com/o/r.git", "git@git.acme.example:o/r.git", false},
		{"https://github.com/o/r.git", "", false},
	}
	for _, tc := range cases {
		if got := SameGitHost(tc.a, tc.b); got != tc.want {
			t.Errorf("SameGitHost(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}
