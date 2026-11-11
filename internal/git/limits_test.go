package git

import (
	"archive/zip"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http/cgi"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testLimits are small enough that a test repository can cross them.
var testLimits = SizeLimits{Clone: 64 << 20, Zip: 16 << 20}

func TestWatchSizeCancelsAGrowingDirectory(t *testing.T) {
	dir := t.TempDir()
	ctx, done := watchSize(context.Background(), dir, 1<<20)

	// Grows the directory like a clone does until the watchdog cancels it.
	chunk := make([]byte, 64<<10)
	stopped := make(chan struct{})
	defer func() { <-stopped }()
	go func() {
		defer close(stopped)
		for i := 0; ctx.Err() == nil; i++ {
			_ = os.WriteFile(filepath.Join(dir, fmt.Sprintf("pack-%d", i)), chunk, 0o644)
			time.Sleep(10 * time.Millisecond)
		}
	}()

	select {
	case <-ctx.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("watchdog did not cancel a directory past the limit")
	}
	if cause := context.Cause(ctx); !errors.Is(cause, ErrCloneTooLarge) {
		t.Errorf("cause = %v, want ErrCloneTooLarge", cause)
	}
	if err := done(); !errors.Is(err, ErrCloneTooLarge) {
		t.Errorf("done() = %v, want ErrCloneTooLarge", err)
	}
}

func TestWatchSizeAllowsADirectoryBelowTheLimit(t *testing.T) {
	dir := t.TempDir()
	ctx, done := watchSize(context.Background(), dir, 1<<20)
	if err := os.WriteFile(filepath.Join(dir, "pack"), make([]byte, 1<<10), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * sizeCheckInterval)
	if ctx.Err() != nil {
		t.Fatalf("watchdog canceled a directory below the limit: %v", context.Cause(ctx))
	}
	if err := done(); err != nil {
		t.Errorf("done() = %v, want nil", err)
	}
}

// A clone that finishes between two ticks is caught by the final measurement.
func TestWatchSizeChecksOnceMoreWhenDone(t *testing.T) {
	dir := t.TempDir()
	_, done := watchSize(context.Background(), dir, 1<<10)
	if err := os.WriteFile(filepath.Join(dir, "pack"), make([]byte, 4<<10), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := done(); !errors.Is(err, ErrCloneTooLarge) {
		t.Errorf("done() = %v, want ErrCloneTooLarge", err)
	}
}

func randomContent(t *testing.T, n int) string {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestWriteArchiveFailsAboveTheLimit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	// Random bytes do not compress, so the zip is larger than the file.
	dir := commitRepo(t, map[string]string{"static/blob.bin": randomContent(t, 64<<10)})
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)

	_, _, err := writeArchive(context.Background(), dir, "acme", "main", 16<<10)
	if !errors.Is(err, ErrZipTooLarge) {
		t.Fatalf("err = %v, want ErrZipTooLarge", err)
	}
	if want := "archive exceeds the zip size limit of 16384 bytes"; !strings.Contains(err.Error(), want) {
		t.Errorf("err = %q, want it to contain %q", err, want)
	}
	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		t.Errorf("left %s in the temp directory", e.Name())
	}

	name, size, err := writeArchive(context.Background(), dir, "acme", "main", 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(name) }()
	if info, err := os.Stat(name); err != nil || info.Size() != size {
		t.Errorf("size = %d, file = %v, %v", size, info, err)
	}
}

// serveRepo serves the repository in dir over https through git's own smart
// http backend, the way a forge would.
func serveRepo(t *testing.T, dir string) (url string, env []string) {
	t.Helper()
	execPath, err := exec.Command("git", "--exec-path").Output()
	if err != nil {
		t.Skip("git not installed")
	}
	srv := httptest.NewTLSServer(&cgi.Handler{
		Path: filepath.Join(strings.TrimSpace(string(execPath)), "git-http-backend"),
		Env:  []string{"GIT_PROJECT_ROOT=" + dir, "GIT_HTTP_EXPORT_ALL=1"},
	})
	t.Cleanup(srv.Close)
	SkipRemoteCheck(t)
	return srv.URL + "/.git", []string{
		"GIT_SSL_NO_VERIFY=1",
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=/dev/null",
	}
}

func TestCloneAndZipWithoutCheckout(t *testing.T) {
	dir := commitRepo(t, map[string]string{
		"acme_addon/__manifest__.py": testManifest,
		"acme_addon/README.md":       "# Acme\n",
	})
	if err := runGit(context.Background(), dir, nil, "tag", "17.0.1.0.0"); err != nil {
		t.Fatal(err)
	}
	url, env := serveRepo(t, dir)

	for _, ref := range []string{"main", "17.0.1.0.0"} {
		t.Run(ref, func(t *testing.T) {
			res, err := CloneAndZip(context.Background(), url, env, ref, "acme_addon", "acme_addon", testLimits)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = os.Remove(res.ZipPath) }()
			if res.Readme != "# Acme\n" || res.Manifest == nil {
				t.Errorf("readme = %q, manifest = %v", res.Readme, res.Manifest)
			}
			zr, err := zip.OpenReader(res.ZipPath)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = zr.Close() }()
			var names []string
			for _, f := range zr.File {
				names = append(names, f.Name)
			}
			if got := strings.Join(names, ","); !strings.Contains(got, "acme_addon/__manifest__.py") {
				t.Errorf("zip holds %s", got)
			}
		})
	}
}

func TestCloneWatchdogStopsALargeClone(t *testing.T) {
	dir := commitRepo(t, map[string]string{"static/blob.bin": randomContent(t, 2<<20)})
	url, env := serveRepo(t, dir)

	tmp := t.TempDir()
	ctx, done := watchSize(context.Background(), tmp, 256<<10)
	cloneErr := runGit(ctx, "", env, "clone", "--depth", "1", "--no-checkout", url, filepath.Join(tmp, "src"))
	if err := done(); !errors.Is(err, ErrCloneTooLarge) {
		t.Errorf("done() = %v (clone: %v), want ErrCloneTooLarge", err, cloneErr)
	}
}
