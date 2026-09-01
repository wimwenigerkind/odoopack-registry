package git

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	odoomanifest "github.com/wimwenigerkind/odoo-manifest"
)

type ArchiveResult struct {
	ZipPath     string
	ContentHash string
	SizeBytes   int64
	Readme      string
	Manifest    *odoomanifest.Manifest
}

func CloneAndZip(url, ref, rootDir, subpath string) (ArchiveResult, error) {
	tmp, err := os.MkdirTemp("", "odoopack-build-*")
	if err != nil {
		return ArchiveResult{}, err
	}
	defer os.RemoveAll(tmp)

	src := filepath.Join(tmp, "src")
	if err := runGit("", "clone", "--depth", "1", "--branch", ref, "--single-branch", url, src); err != nil {
		return ArchiveResult{}, fmt.Errorf("git clone %s@%s: %w", url, ref, err)
	}

	treeish := ref
	if cleaned := strings.Trim(subpath, "/"); cleaned != "" {
		treeish = ref + ":" + cleaned
	}

	outFile, err := os.CreateTemp("", "odoopack-zip-*.zip")
	if err != nil {
		return ArchiveResult{}, err
	}
	outName := outFile.Name()
	_ = outFile.Close()

	keep := false
	defer func() {
		if !keep {
			os.Remove(outName)
		}
	}()

	prefix := strings.TrimSuffix(rootDir, "/") + "/"
	if err := runGit(src, "archive",
		"--format=zip",
		"--prefix="+prefix,
		"-o", outName,
		treeish,
	); err != nil {
		return ArchiveResult{}, fmt.Errorf("git archive %s: %w", treeish, err)
	}

	info, err := os.Stat(outName)
	if err != nil {
		return ArchiveResult{}, err
	}

	hash, err := sha256File(outName)
	if err != nil {
		return ArchiveResult{}, err
	}

	keep = true
	return ArchiveResult{
		ZipPath:     outName,
		ContentHash: hash,
		SizeBytes:   info.Size(),
		Readme:      readReadme(src, subpath),
		Manifest:    readManifest(src, subpath),
	}, nil
}

func CloneAndZipAtSHA(url, sha, rootDir, subpath string) (ArchiveResult, error) {
	tmp, err := os.MkdirTemp("", "odoopack-pinned-*")
	if err != nil {
		return ArchiveResult{}, err
	}
	defer os.RemoveAll(tmp)

	src := filepath.Join(tmp, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		return ArchiveResult{}, err
	}
	if err := runGit(src, "init", "-q"); err != nil {
		return ArchiveResult{}, fmt.Errorf("git init: %w", err)
	}
	if err := runGit(src, "remote", "add", "origin", url); err != nil {
		return ArchiveResult{}, fmt.Errorf("git remote add: %w", err)
	}
	if err := runGit(src, "fetch", "--depth", "1", "origin", sha); err != nil {
		if err := runGit(src, "fetch", "--tags", "origin"); err != nil {
			return ArchiveResult{}, fmt.Errorf("git fetch %s: %w", sha, err)
		}
	}

	treeish := sha
	if cleaned := strings.Trim(subpath, "/"); cleaned != "" {
		treeish = sha + ":" + cleaned
	}

	outFile, err := os.CreateTemp("", "odoopack-zip-*.zip")
	if err != nil {
		return ArchiveResult{}, err
	}
	outName := outFile.Name()
	_ = outFile.Close()

	keep := false
	defer func() {
		if !keep {
			os.Remove(outName)
		}
	}()

	prefix := strings.TrimSuffix(rootDir, "/") + "/"
	if err := runGit(src, "archive",
		"--format=zip",
		"--prefix="+prefix,
		"-o", outName,
		treeish,
	); err != nil {
		return ArchiveResult{}, fmt.Errorf("git archive %s: %w", treeish, err)
	}

	info, err := os.Stat(outName)
	if err != nil {
		return ArchiveResult{}, err
	}
	hash, err := sha256File(outName)
	if err != nil {
		return ArchiveResult{}, err
	}

	keep = true
	return ArchiveResult{
		ZipPath:     outName,
		ContentHash: hash,
		SizeBytes:   info.Size(),
		Readme:      readReadme(src, subpath),
		Manifest:    readManifest(src, subpath),
	}, nil
}

func readReadme(srcDir, subpath string) string {
	dir := srcDir
	if cleaned := strings.Trim(subpath, "/"); cleaned != "" {
		dir = filepath.Join(srcDir, cleaned)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.EqualFold(e.Name(), "README.md") {
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				return ""
			}
			return string(data)
		}
	}
	return ""
}

func readManifest(srcDir, subpath string) *odoomanifest.Manifest {
	dir := srcDir
	if cleaned := strings.Trim(subpath, "/"); cleaned != "" {
		dir = filepath.Join(srcDir, cleaned)
	}
	data, err := os.ReadFile(filepath.Join(dir, "__manifest__.py"))
	if err != nil {
		return nil
	}
	m, err := odoomanifest.Parse(data)
	if err != nil {
		slog.Warn("parse manifest", "dir", dir, "err", err)
		return nil
	}
	return &m
}

func runGit(workdir string, args ...string) error {
	cmd := exec.Command("git", args...)
	if workdir != "" {
		cmd.Dir = workdir
	}
	cmd.Env = append(os.Environ(),
		"GIT_ALLOW_PROTOCOL=https:ssh",
		"GIT_PROTOCOL_FROM_USER=0",
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
