package config

import (
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestParseSize(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want int64
		ok   bool
	}{
		{"100MiB", 100 << 20, true},
		{"1GiB", 1 << 30, true},
		{"512KiB", 512 << 10, true},
		{"2TiB", 2 << 40, true},
		{"1048576", 1 << 20, true},
		{"1048576B", 1 << 20, true},
		{"100mib", 100 << 20, true},
		{"1gib", 1 << 30, true},
		{" 100 MiB ", 100 << 20, true},
		{"0", 0, true},
		{"", 0, false},
		{"MiB", 0, false},
		{"100MB", 0, false},
		{"100M", 0, false},
		{"1.5GiB", 0, false},
		{"-1MiB", 0, false},
		{"abc", 0, false},
		{"9223372036854775807", 1<<63 - 1, true},
		{"9223372036854775808", 0, false},
		{"8388608TiB", 0, false},
		{"8388607TiB", 8388607 << 40, true},
	} {
		got, err := ParseSize(tc.in)
		if (err == nil) != tc.ok || got != tc.want {
			t.Errorf("ParseSize(%q) = %d, %v, want %d, ok=%v", tc.in, got, err, tc.want, tc.ok)
		}
	}
}

func TestLoadLimits(t *testing.T) {
	for _, tc := range []struct {
		name    string
		set     map[string]any
		wantErr string
	}{
		{"defaults", nil, ""},
		{"overrides from yaml or env", map[string]any{
			"limits.zip_max_size": "512KiB", "limits.clone_max_size": 1 << 20,
			"limits.tags_per_sync": "5", "limits.branches_per_sync": 2,
			"limits.sync_interval": "0s", "limits.job_timeout": "1m",
		}, ""},
		{"unparsable size", map[string]any{"limits.zip_max_size": "100MB"}, "limits.zip_max_size"},
		{"zero size", map[string]any{"limits.clone_max_size": "0"}, "limits.clone_max_size must be greater than zero"},
		{"unparsable count", map[string]any{"limits.tags_per_sync": "many"}, "limits.tags_per_sync"},
		{"zero count", map[string]any{"limits.branches_per_sync": 0}, "limits.branches_per_sync must be greater than zero"},
		{"negative count", map[string]any{"limits.tags_per_sync": -1}, "limits.tags_per_sync must be greater than zero"},
		{"unparsable duration", map[string]any{"limits.sync_interval": "60"}, "limits.sync_interval"},
		{"negative interval", map[string]any{"limits.sync_interval": "-1m"}, "must not be negative"},
		{"zip above clone", map[string]any{"limits.zip_max_size": "2GiB"}, "must not exceed limits.clone_max_size"},
		{"short job timeout", map[string]any{"limits.job_timeout": "59s"}, "limits.job_timeout must be at least 1m0s"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := viper.New()
			setLimitDefaults(v)
			for k, val := range tc.set {
				v.Set(k, val)
			}
			l, err := loadLimits(v)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want it to contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tc.set == nil {
				want := Limits{ZipMaxSize: 100 << 20, CloneMaxSize: 1 << 30, TagsPerSync: 50, BranchesPerSync: 10,
					SyncInterval: time.Minute, JobTimeout: 10 * time.Minute}
				if l != want {
					t.Errorf("defaults = %+v, want %+v", l, want)
				}
			} else if l.ZipMaxSize != 512<<10 || l.CloneMaxSize != 1<<20 || l.TagsPerSync != 5 || l.SyncInterval != 0 {
				t.Errorf("overrides = %+v", l)
			}
		})
	}
}

// The env names follow from the replacer in LoadConfig.
func TestLoadLimitsFromEnv(t *testing.T) {
	t.Setenv("LIMITS_ZIP_MAX_SIZE", "50MiB")
	v := viper.New()
	setLimitDefaults(v)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	l, err := loadLimits(v)
	if err != nil {
		t.Fatal(err)
	}
	if l.ZipMaxSize != 50<<20 {
		t.Errorf("ZipMaxSize = %d, want 50 MiB", l.ZipMaxSize)
	}
}
