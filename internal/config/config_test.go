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
