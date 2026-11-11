package worker

import (
	"testing"

	"github.com/wimwenigerkind/odoopack-registry/internal/git"
)

func TestPlanVersions(t *testing.T) {
	tests := []struct {
		name string
		ref  git.Ref
		want string
	}{
		{"canonical release tag", git.Ref{Type: git.RefTag, Name: "19.0.1.0.0"}, "19.0.1.0.0"},
		{"tag with v prefix", git.Ref{Type: git.RefTag, Name: "v19.0.1.0.0"}, "19.0.1.0.0"},
		{"tag with V prefix", git.Ref{Type: git.RefTag, Name: "V19.0.1.0.0"}, "19.0.1.0.0"},
		{"semver tag", git.Ref{Type: git.RefTag, Name: "1.2.3"}, ""},
		{"semver tag with v prefix", git.Ref{Type: git.RefTag, Name: "v1.2.3"}, ""},
		{"tag with leading zero", git.Ref{Type: git.RefTag, Name: "19.0.01.0.0"}, ""},
		{"tag with suffix", git.Ref{Type: git.RefTag, Name: "19.0.1.0.0-rc1"}, ""},
		{"tag named like a dev version", git.Ref{Type: git.RefTag, Name: "dev-main"}, ""},
		{"branch", git.Ref{Type: git.RefBranch, Name: "main"}, "dev-main"},
		{"branch with series and slash", git.Ref{Type: git.RefBranch, Name: "feature/19.0-acme"}, "dev-feature/19.0-acme"},
		{"branch with space", git.Ref{Type: git.RefBranch, Name: "my branch"}, ""},
		{"branch with dot dot", git.Ref{Type: git.RefBranch, Name: "a..b"}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := planVersions([]git.Ref{tc.ref})
			if tc.want == "" {
				if len(got) != 0 {
					t.Fatalf("planVersions(%q) = %q, want skipped", tc.ref.Name, got[0].Version)
				}
				return
			}
			if len(got) != 1 || got[0].Version != tc.want {
				t.Fatalf("planVersions(%q) = %v, want %q", tc.ref.Name, got, tc.want)
			}
		})
	}
}
