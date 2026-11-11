package config

import "testing"

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
