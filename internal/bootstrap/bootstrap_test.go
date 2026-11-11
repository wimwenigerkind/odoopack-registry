package bootstrap

import (
	"testing"

	"github.com/spf13/viper"
)

func TestMemoryStoreNeedsDevMode(t *testing.T) {
	t.Cleanup(viper.Reset)
	for _, tc := range []struct {
		store   string
		devMode bool
		ok      bool
	}{
		{"memory", false, false},
		{"", false, false},
		{"memory", true, true},
		{"", true, true},
	} {
		viper.Reset()
		viper.Set("session.store", tc.store)
		viper.Set("dev_mode", tc.devMode)

		sessions, err := SessionStore()
		if (err == nil) != tc.ok {
			t.Errorf("SessionStore() with store %q, dev_mode %v: err = %v, want ok=%v", tc.store, tc.devMode, err, tc.ok)
		}
		if sessions != nil {
			sessions.Stop()
		}
		state, err := StateStore()
		if (err == nil) != tc.ok {
			t.Errorf("StateStore() with store %q, dev_mode %v: err = %v, want ok=%v", tc.store, tc.devMode, err, tc.ok)
		}
		if state != nil {
			state.Stop()
		}
	}
}
