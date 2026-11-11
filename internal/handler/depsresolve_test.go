package handler

import (
	"slices"
	"testing"

	"github.com/wimwenigerkind/odoopack-registry/internal/models"
)

func TestDependedModules(t *testing.T) {
	for _, tc := range []struct {
		name     string
		versions []models.AddonVersion
		want     []string
	}{
		{name: "no versions"},
		{name: "no depends", versions: []models.AddonVersion{{Version: "dev-main"}}},
		{name: "distinct across versions", versions: []models.AddonVersion{
			{Depends: []string{"base", "acme_essentials"}},
			{},
			{Depends: []string{"acme_essentials", "acme_warranty", "base"}},
		}, want: []string{"base", "acme_essentials", "acme_warranty"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := dependedModules(tc.versions); !slices.Equal(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
