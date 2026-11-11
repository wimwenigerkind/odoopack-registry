package git

import (
	"context"
	"testing"
)

// SkipRemoteCheck lets a test run git against an httptest server, which
// listens on loopback and would fail the host check.
func SkipRemoteCheck(t *testing.T) {
	saved := checkRemote
	checkRemote = func(context.Context, string) error { return nil }
	t.Cleanup(func() { checkRemote = saved })
}
