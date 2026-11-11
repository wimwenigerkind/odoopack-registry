package git

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"slices"
	"testing"
	"time"
)

// A remote that accepts the connection and never answers must not hold a
// build past its context, and the build must not leave its temp directory.
func TestGitStopsAtTheContextDeadline(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	SkipRemoteCheck(t)

	release := make(chan struct{})
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)

	env := []string{
		"GIT_SSL_NO_VERIFY=1",
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=/dev/null",
	}
	url := srv.URL + "/acme/addons.git"
	ops := map[string]func(context.Context) error{
		"ls-remote": func(ctx context.Context) error {
			_, err := LsRemote(ctx, url, env)
			return err
		},
		"clone": func(ctx context.Context) error {
			_, err := CloneAndZip(ctx, url, env, "main", "addons", "", testLimits)
			return err
		},
		"fetch": func(ctx context.Context) error {
			_, err := CloneAndZipAtSHA(ctx, url, env, "0123456789abcdef0123456789abcdef01234567", "addons", "", testLimits)
			return err
		},
	}
	for name, op := range ops {
		t.Run(name, func(t *testing.T) {
			tmp := t.TempDir()
			t.Setenv("TMPDIR", tmp)

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			start := time.Now()
			err := op(ctx)
			if elapsed := time.Since(start); elapsed > 10*time.Second {
				t.Errorf("returned after %s, want shortly after the 1s deadline", elapsed)
			}
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("err = %v, want context.DeadlineExceeded", err)
			}
			entries, err := os.ReadDir(tmp)
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range entries {
				t.Errorf("left %s in the temp directory", e.Name())
			}
		})
	}
}

func TestGitCommandSetsTransferLimits(t *testing.T) {
	cmd := gitCommand(context.Background(), "", nil, "ls-remote")
	for _, want := range []string{"GIT_TERMINAL_PROMPT=0", "GIT_HTTP_LOW_SPEED_LIMIT=1000", "GIT_HTTP_LOW_SPEED_TIME=60"} {
		if !slices.Contains(cmd.Env, want) {
			t.Errorf("env lacks %s", want)
		}
	}
}
