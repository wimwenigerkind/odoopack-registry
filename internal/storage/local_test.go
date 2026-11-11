package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

const objectSize = 64 * 4096

// slowReader yields size bytes of b in small chunks with a pause between
// them, so a concurrent reader gets many chances to see a half written file.
type slowReader struct {
	b    byte
	left int
}

func (r *slowReader) Read(p []byte) (int, error) {
	if r.left == 0 {
		return 0, io.EOF
	}
	n := min(len(p), 4096, r.left)
	for i := range n {
		p[i] = r.b
	}
	r.left -= n
	time.Sleep(200 * time.Microsecond)
	return n, nil
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, errors.New("upstream broke")
}

func newLocalStorage(t *testing.T) *LocalStorage {
	t.Helper()
	s, err := NewLocalStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func readKey(t *testing.T, s *LocalStorage, key string) []byte {
	t.Helper()
	rc, err := s.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("get %s: %v", key, err)
	}
	defer func() { _ = rc.Close() }()
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read %s: %v", key, err)
	}
	return got
}

func TestLocalStoragePutIsAtomic(t *testing.T) {
	s := newLocalStorage(t)
	ctx := context.Background()
	key := "packages/acme/1.0.0.zip"

	done := make(chan struct{})
	var wg sync.WaitGroup
	var reads int
	wg.Go(func() {
		for {
			select {
			case <-done:
				return
			default:
			}
			rc, err := s.Get(ctx, key)
			if errors.Is(err, ErrNotFound) {
				continue
			}
			if err != nil {
				t.Errorf("get: %v", err)
				return
			}
			got, err := io.ReadAll(rc)
			_ = rc.Close()
			if err != nil {
				t.Errorf("read: %v", err)
				return
			}
			reads++
			if len(got) != objectSize || bytes.Count(got, got[:1]) != objectSize {
				t.Errorf("saw a partial object: %d bytes", len(got))
				return
			}
		}
	})

	// The first round writes a new key, the following ones overwrite it.
	for _, b := range []byte("abc") {
		if err := s.Put(ctx, key, &slowReader{b: b, left: objectSize}, PutOptions{}); err != nil {
			t.Fatalf("put %q: %v", b, err)
		}
	}
	close(done)
	wg.Wait()

	if reads == 0 {
		t.Fatal("the reader never saw the object")
	}
	if got := readKey(t, s, key); !bytes.Equal(got, bytes.Repeat([]byte("c"), objectSize)) {
		t.Fatal("the last put did not win")
	}
}

func TestLocalStorageFailedPutKeepsPreviousContent(t *testing.T) {
	s := newLocalStorage(t)
	ctx := context.Background()
	key := "packages/acme/1.0.0.zip"
	old := []byte("previous content")

	if err := s.Put(ctx, key, bytes.NewReader(old), PutOptions{}); err != nil {
		t.Fatal(err)
	}
	r := io.MultiReader(bytes.NewReader([]byte("half of the new")), failingReader{})
	if err := s.Put(ctx, key, r, PutOptions{}); err == nil {
		t.Fatal("put with a failing reader succeeded")
	}

	if got := readKey(t, s, key); !bytes.Equal(got, old) {
		t.Fatalf("got %q, want %q", got, old)
	}
	entries, err := os.ReadDir(filepath.Join(s.root, "packages", "acme"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "1.0.0.zip" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("directory holds %v, want only the key", names)
	}
}

func TestLocalStoragePutFileMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no Unix permission bits")
	}
	s := newLocalStorage(t)
	key := "packages/acme/1.0.0.zip"
	if err := s.Put(context.Background(), key, bytes.NewReader([]byte("x")), PutOptions{}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(s.root, key))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0644 {
		t.Fatalf("mode %v, want 0644", got)
	}
}

func TestLocalStorageRejectsTraversal(t *testing.T) {
	parent := t.TempDir()
	s, err := NewLocalStorage(filepath.Join(parent, "root"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	for _, key := range []string{"../escape.zip", "packages/../../escape.zip", "..", ""} {
		if err := s.Put(ctx, key, bytes.NewReader([]byte("x")), PutOptions{}); err == nil {
			t.Errorf("put %q succeeded", key)
		}
		if _, err := s.Get(ctx, key); err == nil || errors.Is(err, ErrNotFound) {
			t.Errorf("get %q: got %v, want an invalid key error", key, err)
		}
		if err := s.Delete(ctx, key); err == nil || errors.Is(err, ErrNotFound) {
			t.Errorf("delete %q: got %v, want an invalid key error", key, err)
		}
	}

	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "root" {
		t.Fatalf("something was written outside the root: %d entries", len(entries))
	}
}
