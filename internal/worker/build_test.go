package worker

import (
	"fmt"
	"slices"
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
			got, _ := planVersions([]git.Ref{tc.ref}, "main", 50, 10)
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

func TestPlanVersionsKeepsTheNewestRefs(t *testing.T) {
	var refs []git.Ref
	// Older series first and minor versions out of order, so that only a
	// version sort and not the input order finds the newest.
	for _, series := range []string{"17.0", "18.0", "19.0"} {
		for minor := 39; minor >= 0; minor-- {
			refs = append(refs, git.Ref{Type: git.RefTag, Name: fmt.Sprintf("%s.1.%d.0", series, minor)})
		}
	}
	for i := range 12 {
		refs = append(refs, git.Ref{Type: git.RefBranch, Name: fmt.Sprintf("feature-%02d", i)})
	}
	refs = append(refs,
		git.Ref{Type: git.RefBranch, Name: "17.0"},
		git.Ref{Type: git.RefBranch, Name: "19.0"},
		git.Ref{Type: git.RefBranch, Name: "zz-main"},
	)

	desired, skipped := planVersions(refs, "zz-main", 50, 10)
	if len(desired)+len(skipped) != len(refs) {
		t.Fatalf("%d desired + %d skipped, want %d refs", len(desired), len(skipped), len(refs))
	}

	var tags, branches []string
	for _, d := range desired {
		if d.Ref.Type == git.RefTag {
			tags = append(tags, d.Version)
		} else {
			branches = append(branches, d.Version)
		}
	}
	if len(tags) != 50 {
		t.Fatalf("kept %d tags, want 50", len(tags))
	}
	if tags[0] != "19.0.1.39.0" || tags[39] != "19.0.1.0.0" || tags[40] != "18.0.1.39.0" || tags[49] != "18.0.1.30.0" {
		t.Errorf("kept tags %v, want 19.0.1.39.0 down to 18.0.1.30.0", tags)
	}
	wantBranches := []string{"dev-zz-main", "dev-19.0", "dev-17.0", "dev-feature-00", "dev-feature-01",
		"dev-feature-02", "dev-feature-03", "dev-feature-04", "dev-feature-05", "dev-feature-06"}
	if !slices.Equal(branches, wantBranches) {
		t.Errorf("kept branches %v, want %v", branches, wantBranches)
	}
}
