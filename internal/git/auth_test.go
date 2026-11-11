package git_test

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"github.com/wimwenigerkind/odoopack-registry/internal/auth"
	"github.com/wimwenigerkind/odoopack-registry/internal/git"
)

// A failed clone must neither echo the token in its error nor need it in the
// URL: the server sees it only as the Authorization header, and only when it
// is the provider's own host. The test server is Forgejo's base_url, so
// GitHub and Bitbucket must not send their tokens to it.
func TestFailedCloneKeepsTokenOut(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	git.SkipRemoteCheck(t)

	var mu sync.Mutex
	var seen []string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Header.Get("Authorization"))
		mu.Unlock()
		// 403 rather than 401, so git does not fall back to a credential helper.
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer srv.Close()

	forgejo, err := auth.NewForgejoProvider("forgejo", auth.ForgejoConfig{BaseURL: srv.URL}, "")
	if err != nil {
		t.Fatal(err)
	}
	const token = "s3cret-token"
	providers := []struct {
		name     string
		provider auth.IntegrationProvider
		wantAuth string
	}{
		{"github", auth.NewGitHubProvider("github", auth.GitHubConfig{}, ""), ""},
		{"bitbucket", auth.NewBitbucketProvider("bitbucket", auth.BitbucketConfig{}, ""), ""},
		{"forgejo", forgejo, token + ":"},
	}
	for _, p := range providers {
		t.Run(p.name, func(t *testing.T) {
			remote := p.provider.AuthenticateGitRemote(srv.URL+"/o/r.git", token)
			env := append(remote.Env,
				"GIT_SSL_NO_VERIFY=1",
				"GIT_TERMINAL_PROMPT=0",
				"GIT_CONFIG_NOSYSTEM=1",
				"GIT_CONFIG_GLOBAL=/dev/null",
			)

			mu.Lock()
			seen = nil
			mu.Unlock()

			ctx := context.Background()

			_, lsErr := git.LsRemote(ctx, remote.URL, env)
			_, cloneErr := git.CloneAndZip(ctx, remote.URL, env, "main", "r", "")
			_, fetchErr := git.CloneAndZipAtSHA(ctx, remote.URL, env, "0123456789abcdef0123456789abcdef01234567", "r", "")
			for _, err := range []error{lsErr, cloneErr, fetchErr} {
				if err == nil {
					t.Fatal("want an error from a forbidden remote")
				}
				if strings.Contains(err.Error(), token) {
					t.Errorf("error contains the token: %v", err)
				}
			}

			want := ""
			if p.wantAuth != "" {
				want = "Basic " + base64.StdEncoding.EncodeToString([]byte(p.wantAuth))
			}
			mu.Lock()
			defer mu.Unlock()
			if len(seen) == 0 {
				t.Fatal("server saw no request")
			}
			for _, got := range seen {
				if got != want {
					t.Errorf("Authorization %q, want %q", got, want)
				}
			}
		})
	}
}
