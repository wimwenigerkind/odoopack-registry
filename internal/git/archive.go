package git

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	odoomanifest "github.com/wimwenigerkind/odoo-manifest"
	"github.com/wimwenigerkind/odoopack-registry/internal/validate"
)

type ArchiveResult struct {
	ZipPath     string
	ContentHash string
	SizeBytes   int64
	Readme      string
	Manifest    *odoomanifest.Manifest
}

func CloneAndZip(ctx context.Context, url string, env []string, ref, rootDir, subpath string, limits SizeLimits) (ArchiveResult, error) {
	tmp, err := os.MkdirTemp("", "odoopack-build-*")
	if err != nil {
		return ArchiveResult{}, err
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	src := filepath.Join(tmp, "src")
	if err := checkRemote(ctx, url); err != nil {
		return ArchiveResult{}, err
	}
	// Without a worktree the clone holds only compressed objects, so a small
	// pack cannot expand on disk at checkout; archive and readBlob read the
	// object database anyway.
	cloneCtx, cloneDone := watchSize(ctx, tmp, limits.Clone)
	err = runGit(cloneCtx, "", env, "clone", "--depth", "1", "--branch", ref, "--single-branch", "--no-checkout", url, src)
	if sizeErr := cloneDone(); sizeErr != nil {
		err = sizeErr
	}
	if err != nil {
		return ArchiveResult{}, fmt.Errorf("git clone %s@%s: %w", url, ref, err)
	}

	treeish := ref
	if cleaned := strings.Trim(subpath, "/"); cleaned != "" {
		treeish = ref + ":" + cleaned
	}

	outName, size, err := writeArchive(ctx, src, rootDir, treeish, limits.Zip)
	if err != nil {
		return ArchiveResult{}, err
	}
	keep := false
	defer func() {
		if !keep {
			_ = os.Remove(outName)
		}
	}()

	hash, err := sha256File(outName)
	if err != nil {
		return ArchiveResult{}, err
	}

	keep = true
	return ArchiveResult{
		ZipPath:     outName,
		ContentHash: hash,
		SizeBytes:   size,
		Readme:      readReadme(ctx, src, ref, subpath),
		Manifest:    readManifest(ctx, src, ref, subpath),
	}, nil
}

func CloneAndZipAtSHA(ctx context.Context, url string, env []string, sha, rootDir, subpath string, limits SizeLimits) (ArchiveResult, error) {
	tmp, err := os.MkdirTemp("", "odoopack-pinned-*")
	if err != nil {
		return ArchiveResult{}, err
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	src := filepath.Join(tmp, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		return ArchiveResult{}, err
	}
	if err := runGit(ctx, src, nil, "init", "-q"); err != nil {
		return ArchiveResult{}, fmt.Errorf("git init: %w", err)
	}
	if err := runGit(ctx, src, nil, "remote", "add", "origin", url); err != nil {
		return ArchiveResult{}, fmt.Errorf("git remote add: %w", err)
	}
	if err := checkRemote(ctx, url); err != nil {
		return ArchiveResult{}, err
	}
	fetchCtx, fetchDone := watchSize(ctx, tmp, limits.Clone)
	err = fetchSHA(fetchCtx, src, env, url, sha)
	if sizeErr := fetchDone(); sizeErr != nil {
		err = fmt.Errorf("git fetch %s: %w", sha, sizeErr)
	}
	if err != nil {
		return ArchiveResult{}, err
	}

	treeish := sha
	if cleaned := strings.Trim(subpath, "/"); cleaned != "" {
		treeish = sha + ":" + cleaned
	}

	outName, size, err := writeArchive(ctx, src, rootDir, treeish, limits.Zip)
	if err != nil {
		return ArchiveResult{}, err
	}
	keep := false
	defer func() {
		if !keep {
			_ = os.Remove(outName)
		}
	}()

	hash, err := sha256File(outName)
	if err != nil {
		return ArchiveResult{}, err
	}

	keep = true
	return ArchiveResult{
		ZipPath:     outName,
		ContentHash: hash,
		SizeBytes:   size,
		Readme:      readReadme(ctx, src, sha, subpath),
		Manifest:    readManifest(ctx, src, sha, subpath),
	}, nil
}

func fetchSHA(ctx context.Context, src string, env []string, url, sha string) error {
	if err := runGit(ctx, src, env, "fetch", "--depth", "1", "origin", sha); err != nil {
		if err := checkRemote(ctx, url); err != nil {
			return err
		}
		if err := runGit(ctx, src, env, "fetch", "--tags", "origin"); err != nil {
			return fmt.Errorf("git fetch %s: %w", sha, err)
		}
	}
	return nil
}

// writeArchive zips treeish from the repository in src into a new temp file
// and returns its path. The zip streams through a cappedWriter, so an archive
// above limit fails without ever being written in full.
func writeArchive(ctx context.Context, src, rootDir, treeish string, limit int64) (string, int64, error) {
	outFile, err := os.CreateTemp("", "odoopack-zip-*.zip")
	if err != nil {
		return "", 0, err
	}
	outName := outFile.Name()
	w := &cappedWriter{f: outFile, limit: limit}

	prefix := strings.TrimSuffix(rootDir, "/") + "/"
	err = streamGit(ctx, src, nil, w, "archive", "--format=zip", "--prefix="+prefix, treeish)
	if closeErr := outFile.Close(); err == nil {
		err = closeErr
	}
	if w.over {
		err = fmt.Errorf("%w of %s", ErrZipTooLarge, formatBytes(limit))
	}
	if err != nil {
		_ = os.Remove(outName)
		return "", 0, fmt.Errorf("git archive %s: %w", treeish, err)
	}
	return outName, w.written, nil
}

// maxMetadataBytes caps the README and manifest read from a repository; a
// larger file is dropped rather than truncated.
const maxMetadataBytes = 1 << 20

func readReadme(ctx context.Context, srcDir, commitish, subpath string) string {
	data := readBlob(ctx, srcDir, commitish, subpath, func(name string) bool {
		return strings.EqualFold(name, "README.md")
	})
	return string(data)
}

func readManifest(ctx context.Context, srcDir, commitish, subpath string) *odoomanifest.Manifest {
	data := readBlob(ctx, srcDir, commitish, subpath, func(name string) bool {
		return name == "__manifest__.py"
	})
	if data == nil {
		return nil
	}
	m, err := odoomanifest.Parse(data)
	if err != nil {
		slog.Warn("parse manifest", "commit", commitish, "subpath", subpath, "err", err)
		return nil
	}
	return &m
}

// readBlob returns the first regular file in the tree at commitish:subpath
// whose name matches. It reads from the object database instead of the
// worktree so that a symlink in the repository can never point the reader
// at a file outside of it; symlink entries (mode 120000) count as absent.
func readBlob(ctx context.Context, srcDir, commitish, subpath string, match func(name string) bool) []byte {
	treeish := commitish
	if cleaned := strings.Trim(subpath, "/"); cleaned != "" {
		treeish = commitish + ":" + cleaned
	}
	out, err := outputGit(ctx, srcDir, nil, "ls-tree", "-z", "--long", treeish)
	if err != nil {
		return nil
	}
	for _, entry := range strings.Split(string(out), "\x00") {
		meta, name, ok := strings.Cut(entry, "\t")
		if !ok || !match(name) {
			continue
		}
		fields := strings.Fields(meta)
		if len(fields) != 4 || fields[1] != "blob" {
			continue
		}
		if fields[0] != "100644" && fields[0] != "100755" {
			continue
		}
		size, err := strconv.ParseInt(fields[3], 10, 64)
		if err != nil || size > maxMetadataBytes {
			return nil
		}
		data, err := outputGit(ctx, srcDir, nil, "cat-file", "blob", fields[2])
		if err != nil {
			return nil
		}
		return data
	}
	return nil
}

func runGit(ctx context.Context, workdir string, env []string, args ...string) error {
	_, err := outputGit(ctx, workdir, env, args...)
	return err
}

// checkRemote runs right before every clone, fetch and ls-remote. git
// resolves the name again on its own, so a DNS rebinding between this check
// and git's connect still gets through; the egress NetworkPolicy of the
// deployment is the second line against that.
var checkRemote = validate.GitHost

// A remote that sends less than lowSpeedLimit bytes per second for
// lowSpeedTime seconds aborts the transfer, so a trickling server cannot hold
// a build until the context deadline. Over ssh only the context applies.
const (
	lowSpeedLimit = 1000
	lowSpeedTime  = 60
)

// waitDelay bounds how long Wait blocks on output pipes after git was killed.
const waitDelay = 5 * time.Second

func gitCommand(ctx context.Context, workdir string, env []string, args ...string) *exec.Cmd {
	// A redirect would send git to a host that never passed checkRemote.
	cmd := exec.CommandContext(ctx, "git", append([]string{"-c", "http.followRedirects=false"}, args...)...)
	if workdir != "" {
		cmd.Dir = workdir
	}
	cmd.Env = append(os.Environ(),
		"GIT_ALLOW_PROTOCOL=https:ssh",
		"GIT_PROTOCOL_FROM_USER=0",
		"GIT_TERMINAL_PROMPT=0",
		"GIT_HTTP_LOW_SPEED_LIMIT="+strconv.Itoa(lowSpeedLimit),
		"GIT_HTTP_LOW_SPEED_TIME="+strconv.Itoa(lowSpeedTime),
	)
	cmd.Env = append(cmd.Env, env...)
	killProcessGroup(cmd)
	cmd.WaitDelay = waitDelay
	return cmd
}

func outputGit(ctx context.Context, workdir string, env []string, args ...string) ([]byte, error) {
	var out bytes.Buffer
	if err := streamGit(ctx, workdir, env, &out, args...); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func streamGit(ctx context.Context, workdir string, env []string, stdout io.Writer, args ...string) error {
	cmd := gitCommand(ctx, workdir, env, args...)
	var stderr bytes.Buffer
	cmd.Stdout = stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// A killed git reports only "signal: killed"; the context says why,
		// and its cause names the limit when the clone watchdog killed it.
		if ctx.Err() != nil {
			return fmt.Errorf("%w: %w", context.Cause(ctx), err)
		}
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
