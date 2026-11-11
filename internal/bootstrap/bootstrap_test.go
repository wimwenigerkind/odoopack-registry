package bootstrap

import (
	"encoding/base64"
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

func TestIntegrationCipher(t *testing.T) {
	validKey := base64.StdEncoding.EncodeToString(make([]byte, 32))
	cases := []struct {
		name       string
		settings   map[string]any
		wantCipher bool
		wantErr    bool
	}{
		{"no provider, no key", nil, false, false},
		{"login only provider, no key", map[string]any{
			"auth.acme.type": "github", "auth.acme.allow_login": true,
		}, false, false},
		{"git integration, no key", map[string]any{
			"auth.acme.type": "forgejo", "auth.acme.allow_git_integration": true,
		}, false, true},
		{"git integration, valid key", map[string]any{
			"auth.acme.type": "github", "auth.acme.allow_git_integration": true,
			"integrations.encryption_key": validKey,
		}, true, false},
		{"git integration, short key", map[string]any{
			"auth.acme.type": "bitbucket", "auth.acme.allow_git_integration": true,
			"integrations.encryption_key": base64.StdEncoding.EncodeToString(make([]byte, 16)),
		}, false, true},
		{"no provider, valid key", map[string]any{"integrations.encryption_key": validKey}, true, false},
		{"no provider, invalid key", map[string]any{"integrations.encryption_key": "not base64!"}, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			viper.Reset()
			t.Cleanup(viper.Reset)
			for k, v := range tc.settings {
				viper.Set(k, v)
			}
			c, err := IntegrationCipher()
			if (err != nil) != tc.wantErr {
				t.Fatalf("err %v, want error %v", err, tc.wantErr)
			}
			if (c != nil) != tc.wantCipher {
				t.Errorf("cipher %v, want one %v", c, tc.wantCipher)
			}
		})
	}
}
