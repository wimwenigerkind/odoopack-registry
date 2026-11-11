package apitoken

import (
	"regexp"
	"testing"
	"time"
)

func TestGenerate(t *testing.T) {
	a, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	b, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	format := regexp.MustCompile(`^odp_[0-9a-f]{64}$`)
	if !format.MatchString(a) {
		t.Errorf("token %q does not match the format the CLI uses", a)
	}
	if a == b {
		t.Error("two generated tokens are equal")
	}
}

func TestHash(t *testing.T) {
	// sha256("abc") from FIPS 180-2.
	want := "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got := Hash("abc"); got != want {
		t.Errorf("Hash(abc) = %s, want %s", got, want)
	}
	if Hash("odp_a") == Hash("odp_b") {
		t.Error("different tokens share a hash")
	}
}

func TestDisplayParts(t *testing.T) {
	tests := []struct {
		plain, prefix, lastFour string
	}{
		{"odp_0123456789abcdef", "odp_0123", "cdef"},
		{"abc", "abc", "abc"},
		{"", "", ""},
	}
	for _, tt := range tests {
		prefix, lastFour := DisplayParts(tt.plain)
		if prefix != tt.prefix || lastFour != tt.lastFour {
			t.Errorf("DisplayParts(%q) = %q, %q, want %q, %q", tt.plain, prefix, lastFour, tt.prefix, tt.lastFour)
		}
	}
}

func TestExpired(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Second)
	future := now.Add(time.Second)
	tests := []struct {
		name      string
		expiresAt *time.Time
		want      bool
	}{
		{"never expires", nil, false},
		{"expired", &past, true},
		{"expires now", &now, true},
		{"still valid", &future, false},
	}
	for _, tt := range tests {
		if got := Expired(tt.expiresAt, now); got != tt.want {
			t.Errorf("%s: Expired = %v, want %v", tt.name, got, tt.want)
		}
	}
}
