package database

import (
	"testing"

	"github.com/wimwenigerkind/odoopack-registry/internal/apitoken"
)

// The migrated row must be found by the same hash the bearer lookup computes.
func TestLegacyTokenColumns(t *testing.T) {
	const legacy = "odp_fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	got := legacyTokenColumns(legacy)
	want := map[string]any{
		"token_hash": apitoken.Hash(legacy),
		"prefix":     "odp_fedc",
		"last_four":  "3210",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %v, want %v", k, got[k], v)
		}
	}
	if len(got) != len(want) {
		t.Errorf("unexpected columns: %v", got)
	}
}
