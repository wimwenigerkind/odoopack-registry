package config

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Limits bounds what a single user can make the registry do. Web and worker
// load it once at start; see setLimitDefaults for the defaults and why.
type Limits struct {
	ZipMaxSize      int64
	CloneMaxSize    int64
	TagsPerSync     int
	BranchesPerSync int
	// SyncInterval is the minimum time between two manual syncs of one
	// addon; 0 turns that limit off.
	SyncInterval time.Duration
	JobTimeout   time.Duration
}

func setLimitDefaults(v *viper.Viper) {
	// A typical Odoo addon zips to a few MiB; the large ones that bundle JS
	// libraries, fonts and images reach a few tens of MiB. Above 100 MiB an
	// archive is almost certainly committed build output or a data dump.
	v.SetDefault("limits.zip_max_size", "100MiB")
	// The clone is shallow and has no worktree, so this is about the
	// compressed size of the whole repository at one commit. Each worker
	// needs worker.count times this in free temp space.
	v.SetDefault("limits.clone_max_size", "1GiB")
	// Fifty releases cover years of an addon across several Odoo series;
	// versions built by earlier syncs stay published when they fall out.
	v.SetDefault("limits.tags_per_sync", 50)
	v.SetDefault("limits.branches_per_sync", 10)
	v.SetDefault("limits.sync_interval", "1m")
	// The chart derives the worker's termination grace period from this, so
	// a shutdown lets the running job finish instead of killing it halfway.
	v.SetDefault("limits.job_timeout", "10m")
}

// minJobTimeout leaves a sync job enough time for at least one clone.
const minJobTimeout = time.Minute

func LoadLimits() (Limits, error) {
	return loadLimits(viper.GetViper())
}

func loadLimits(v *viper.Viper) (Limits, error) {
	var l Limits
	var errs []error
	size := func(key string) int64 {
		n, err := ParseSize(fmt.Sprint(v.Get(key)))
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", key, err))
		} else if n <= 0 {
			errs = append(errs, fmt.Errorf("%s must be greater than zero", key))
		}
		return n
	}
	count := func(key string) int {
		n, err := strconv.Atoi(strings.TrimSpace(fmt.Sprint(v.Get(key))))
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %q is not a whole number", key, v.Get(key)))
		} else if n <= 0 {
			errs = append(errs, fmt.Errorf("%s must be greater than zero", key))
		}
		return n
	}
	duration := func(key string) time.Duration {
		d, err := time.ParseDuration(strings.TrimSpace(fmt.Sprint(v.Get(key))))
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", key, err))
		}
		return d
	}

	l.ZipMaxSize = size("limits.zip_max_size")
	l.CloneMaxSize = size("limits.clone_max_size")
	l.TagsPerSync = count("limits.tags_per_sync")
	l.BranchesPerSync = count("limits.branches_per_sync")
	l.SyncInterval = duration("limits.sync_interval")
	l.JobTimeout = duration("limits.job_timeout")
	if len(errs) > 0 {
		return Limits{}, errors.Join(errs...)
	}

	// The zip is built from the clone, so a larger zip limit never applies.
	if l.ZipMaxSize > l.CloneMaxSize {
		errs = append(errs, fmt.Errorf("limits.zip_max_size (%d bytes) must not exceed limits.clone_max_size (%d bytes)", l.ZipMaxSize, l.CloneMaxSize))
	}
	if l.SyncInterval < 0 {
		errs = append(errs, fmt.Errorf("limits.sync_interval must not be negative, got %s", l.SyncInterval))
	}
	if l.JobTimeout < minJobTimeout {
		errs = append(errs, fmt.Errorf("limits.job_timeout must be at least %s, got %s", minJobTimeout, l.JobTimeout))
	}
	if len(errs) > 0 {
		return Limits{}, errors.Join(errs...)
	}
	return l, nil
}

// ParseSize reads a byte count with an optional binary unit: "512KiB",
// "100MiB", "1GiB" or plain bytes like "1048576" or "1048576B", case
// insensitive. viper's GetSizeInBytes reads "100MiB" as 0 without an error,
// and decimal units like "MB" are refused because readers disagree on
// whether they mean 1000 or 1024.
func ParseSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	digits := len(s) - len(strings.TrimLeft(s, "0123456789"))
	num, unit := s[:digits], strings.ToLower(strings.TrimSpace(s[digits:]))

	var shift uint
	switch unit {
	case "", "b":
	case "kib":
		shift = 10
	case "mib":
		shift = 20
	case "gib":
		shift = 30
	case "tib":
		shift = 40
	default:
		return 0, fmt.Errorf("size %q: unit must be B, KiB, MiB, GiB or TiB", s)
	}
	if num == "" {
		return 0, fmt.Errorf("size %q: missing number", s)
	}
	n, err := strconv.ParseInt(num, 10, 64)
	if err != nil || n > math.MaxInt64>>shift {
		return 0, fmt.Errorf("size %q is too large", s)
	}
	return n << shift, nil
}
