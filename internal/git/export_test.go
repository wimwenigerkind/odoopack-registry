package git

import "testing"

// SkipRemoteCheck lets a test run git against an httptest server, which
// listens on loopback and would fail the host check.
func SkipRemoteCheck(t *testing.T) {
	saved := checkRemote
	checkRemote = func(string) error { return nil }
	t.Cleanup(func() { checkRemote = saved })
}
