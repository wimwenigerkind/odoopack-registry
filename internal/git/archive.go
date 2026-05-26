package git

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

type ArchiveResult struct {
	ZipPath     string
	ContentHash string
	SizeBytes   int64
}

func CloneAndZip(url, ref, rootDir string) (ArchiveResult, error) {
	tmp, err := os.MkdirTemp("", "odoopack-build-*")
	if err != nil {
		return ArchiveResult{}, err
	}
	defer os.RemoveAll(tmp)

	src := filepath.Join(tmp, "src")
	cmd := exec.Command("git", "clone", "--depth", "1", "--branch", ref, "--single-branch", url, src)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return ArchiveResult{}, fmt.Errorf("git clone %s@%s: %w: %s", url, ref, err, strings.TrimSpace(stderr.String()))
	}
	if err := os.RemoveAll(filepath.Join(src, ".git")); err != nil {
		return ArchiveResult{}, err
	}

	outFile, err := os.CreateTemp("", "odoopack-zip-*.zip")
	if err != nil {
		return ArchiveResult{}, err
	}
	keep := false
	defer func() {
		outFile.Close()
		if !keep {
			os.Remove(outFile.Name())
		}
	}()

	hasher := sha256.New()
	counter := &countingWriter{}
	mw := io.MultiWriter(outFile, hasher, counter)
	zw := zip.NewWriter(mw)

	if err := writeZipTree(zw, src, rootDir); err != nil {
		return ArchiveResult{}, err
	}
	if err := zw.Close(); err != nil {
		return ArchiveResult{}, err
	}
	if err := outFile.Close(); err != nil {
		return ArchiveResult{}, err
	}

	keep = true
	return ArchiveResult{
		ZipPath:     outFile.Name(),
		ContentHash: hex.EncodeToString(hasher.Sum(nil)),
		SizeBytes:   counter.n,
	}, nil
}

func writeZipTree(zw *zip.Writer, root, prefix string) error {
	return filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		zipName := path.Join(prefix, filepath.ToSlash(rel))
		info, err := d.Info()
		if err != nil {
			return err
		}
		if d.IsDir() {
			_, err := zw.Create(zipName + "/")
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = zipName
		header.Method = zip.Deflate
		w, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(w, f)
		return err
	})
}

type countingWriter struct{ n int64 }

func (c *countingWriter) Write(p []byte) (int, error) {
	c.n += int64(len(p))
	return len(p), nil
}
