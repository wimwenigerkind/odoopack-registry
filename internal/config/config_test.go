package config

import (
	"testing"

	"github.com/spf13/viper"
)

func TestValidateInstanceMode(t *testing.T) {
	for _, tc := range []struct {
		mode string
		ok   bool
	}{
		{"public", true},
		{"private", true},
		// viper fills in the "public" default when the key is unset, so an
		// empty value here was set on purpose and is as wrong as a typo.
		{"", false},
		{"privat", false},
		{"Private", false},
		{"PUBLIC", false},
		{" private", false},
	} {
		err := ValidateInstanceMode(tc.mode)
		if (err == nil) != tc.ok {
			t.Errorf("ValidateInstanceMode(%q) = %v, want ok=%v", tc.mode, err, tc.ok)
		}
	}
}

func TestCookieSecure(t *testing.T) {
	yes, no := true, false
	for _, tc := range []struct {
		name         string
		baseURL      string
		explicit     *bool
		wantSecure   bool
		wantInsecure bool
	}{
		{"unset https", "https://packages.example.com", nil, true, false},
		{"unset https upper case scheme", "HTTPS://packages.example.com", nil, true, false},
		{"unset http", "http://localhost:6969", nil, false, false},
		{"unset unparsable", "://", nil, false, false},
		{"explicit true on http", "http://localhost:6969", &yes, true, false},
		{"explicit false on http", "http://localhost:6969", &no, false, false},
		{"explicit true on https", "https://packages.example.com", &yes, true, false},
		{"explicit false on https", "https://packages.example.com", &no, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			secure, insecure := CookieSecure(tc.baseURL, tc.explicit)
			if secure != tc.wantSecure || insecure != tc.wantInsecure {
				t.Errorf("CookieSecure(%q) = (%v, %v), want (%v, %v)", tc.baseURL, secure, insecure, tc.wantSecure, tc.wantInsecure)
			}
		})
	}
}

func TestAutoMigrate(t *testing.T) {
	cases := []struct {
		name        string
		autoMigrate any
		env         string
		devMode     bool
		want        bool
	}{
		{"unset, production", nil, "", false, false},
		{"unset, dev mode", nil, "", true, true},
		{"explicit false wins over dev mode", false, "", true, false},
		{"explicit true without dev mode", true, "", false, true},
		{"env false wins over dev mode", nil, "false", true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			viper.Reset()
			t.Cleanup(viper.Reset)
			viper.AutomaticEnv()
			viper.Set("dev_mode", tc.devMode)
			if tc.autoMigrate != nil {
				viper.Set("auto_migrate", tc.autoMigrate)
			}
			if tc.env != "" {
				t.Setenv("AUTO_MIGRATE", tc.env)
			}
			if got := AutoMigrate(); got != tc.want {
				t.Errorf("AutoMigrate() = %v, want %v", got, tc.want)
			}
		})
	}
}
