package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type LocalStorage struct {
	root string
}

func NewLocalStorage(root string) (*LocalStorage, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0755); err != nil {
		return nil, err
	}
	return &LocalStorage{root: abs}, nil
}

func (s *LocalStorage) resolve(key string) (string, error) {
	if key == "" || strings.Contains(key, "..") {
		return "", fmt.Errorf("storage: invalid key %q", key)
	}
	clean := filepath.Clean(filepath.Join(s.root, key))
	if !strings.HasPrefix(clean, s.root+string(os.PathSeparator)) && clean != s.root {
		return "", fmt.Errorf("storage: key escapes root")
	}
	return clean, nil
}

func (s *LocalStorage) Put(_ context.Context, key string, r io.Reader, _ PutOptions) error {
	path, err := s.resolve(key)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	// Readers may open the key while a Put runs, so the content goes to a
	// temp file first and only a rename makes it visible. The pattern starts
	// with a dot and ends in .tmp, which no key the registry builds does.
	f, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if err := writeFile(f, r); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return syncDir(dir)
}

func writeFile(f *os.File, r io.Reader) error {
	_, err := io.Copy(f, r)
	// CreateTemp creates 0600; 0644 is what os.Create gave under the usual
	// umask of 022, so objects stay readable by other users as before.
	if err == nil {
		err = f.Chmod(0644)
	}
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}

// syncDir makes the rename itself survive a crash. Windows cannot sync a
// directory handle, so it is skipped there.
func syncDir(dir string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	if err := d.Sync(); err != nil {
		_ = d.Close()
		return err
	}
	return d.Close()
}

func (s *LocalStorage) Get(_ context.Context, key string) (io.ReadCloser, error) {
	path, err := s.resolve(key)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	return f, err
}

func (s *LocalStorage) Delete(_ context.Context, key string) error {
	path, err := s.resolve(key)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return ErrNotFound
	}
	return err
}
