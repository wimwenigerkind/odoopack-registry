package git

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SizeLimits bounds one build: Clone what a clone or fetch may write to disk,
// Zip the archive of the version.
type SizeLimits struct {
	Clone int64
	Zip   int64
}

// sizeCheckInterval is how often the clone watchdog measures the directory.
// git can write at most this interval times the network bandwidth past the
// cap before it is killed.
const sizeCheckInterval = 250 * time.Millisecond

var (
	ErrCloneTooLarge = errors.New("repository exceeds the clone size limit")
	ErrZipTooLarge   = errors.New("archive exceeds the zip size limit")
)

// watchSize returns a context that is canceled with ErrCloneTooLarge once dir
// holds more than limit bytes. git has no option that bounds what a clone
// writes (a blob filter would leave the archive without its files), so the
// watchdog measures the directory while git runs and the cancellation kills
// git's process group. done stops the watchdog, measures one last time and
// returns the size error if the limit was crossed, also when that happened
// after the last tick.
func watchSize(ctx context.Context, dir string, limit int64) (watched context.Context, done func() error) {
	watched, cancel := context.WithCancelCause(ctx)
	tooLarge := func() error {
		size := dirSize(dir)
		if size <= limit {
			return nil
		}
		return fmt.Errorf("%w of %s (%s written)", ErrCloneTooLarge, formatBytes(limit), formatBytes(size))
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		t := time.NewTicker(sizeCheckInterval)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-watched.Done():
				return
			case <-t.C:
				if err := tooLarge(); err != nil {
					cancel(err)
					return
				}
			}
		}
	}()

	return watched, func() error {
		close(stop)
		wg.Wait()
		defer cancel(nil)
		if cause := context.Cause(watched); errors.Is(cause, ErrCloneTooLarge) {
			return cause
		}
		return tooLarge()
	}
}

// dirSize sums the regular files below dir. git renames and deletes temporary
// pack files while it runs, so entries that vanish during the walk are skipped.
func dirSize(dir string) int64 {
	var total int64
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

// cappedWriter fails the write that would take the file past limit, which
// closes the pipe git archive writes into and stops it.
type cappedWriter struct {
	f       *os.File
	written int64
	limit   int64
	over    bool
}

func (w *cappedWriter) Write(p []byte) (int, error) {
	if w.written+int64(len(p)) > w.limit {
		w.over = true
		return 0, ErrZipTooLarge
	}
	n, err := w.f.Write(p)
	w.written += int64(n)
	return n, err
}

func formatBytes(n int64) string {
	if n >= 1<<20 {
		return fmt.Sprintf("%d MiB", n>>20)
	}
	return fmt.Sprintf("%d bytes", n)
}
