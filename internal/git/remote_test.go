package git

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/wimwenigerkind/odoopack-registry/internal/validate"
)

func TestGitCommandDisablesRedirects(t *testing.T) {
	cmd := gitCommand(context.Background(), "", nil, "ls-remote", "https://git.example.com/acme/addons.git")
	want := []string{"git", "-c", "http.followRedirects=false", "ls-remote", "https://git.example.com/acme/addons.git"}
	if !slices.Equal(cmd.Args, want) {
		t.Errorf("args %q, want %q", cmd.Args, want)
	}
}

func TestNetworkOperationsCheckTheRemote(t *testing.T) {
	saved := checkRemote
	var checked []string
	checkRemote = func(_ context.Context, url string) error {
		checked = append(checked, url)
		return validate.ErrHostNotAllowed
	}
	t.Cleanup(func() { checkRemote = saved })

	const url = "https://git.example.com/acme/addons.git"
	ctx := context.Background()
	_, lsErr := LsRemote(ctx, url, nil)
	_, cloneErr := CloneAndZip(ctx, url, nil, "main", "r", "", testLimits)
	_, fetchErr := CloneAndZipAtSHA(ctx, url, nil, "0123456789abcdef0123456789abcdef01234567", "r", "", testLimits)
	for _, err := range []error{lsErr, cloneErr, fetchErr} {
		if !errors.Is(err, validate.ErrHostNotAllowed) {
			t.Errorf("err = %v, want ErrHostNotAllowed", err)
		}
	}
	if len(checked) != 3 {
		t.Errorf("checked %d remotes, want 3", len(checked))
	}
}

func TestRedirectIsNotFollowed(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	SkipRemoteCheck(t)

	var hits atomic.Int32
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer target.Close()
	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+r.URL.RequestURI(), http.StatusFound)
	}))
	defer origin.Close()

	env := []string{
		"GIT_SSL_NO_VERIFY=1",
		"GIT_TERMINAL_PROMPT=0",
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=/dev/null",
	}
	if _, err := LsRemote(context.Background(), origin.URL+"/acme/addons.git", env); err == nil {
		t.Fatal("want an error from a redirecting remote")
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("redirect target saw %d requests, want 0", n)
	}
}
