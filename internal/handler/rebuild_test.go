package handler

import (
	"reflect"
	"testing"
)

// While dev-main rebuilds, and after the rebuild failed, the row stays ready
// at the last good build and the registry keeps listing that build.
func TestRegistryResponseListsVersionDuringRebuild(t *testing.T) {
	addon := healthCheckAddon()
	want := registryResponse(addon, contractBaseURL)

	dev := &addon.Versions[1]
	dev.BuildingRef = "0d1f2b3c4a5e6f708192a3b4c5d6e7f809182736"
	if got := registryResponse(addon, contractBaseURL); !reflect.DeepEqual(got, want) {
		t.Errorf("during the rebuild = %+v, want %+v", got, want)
	}

	dev.BuildingRef, dev.BuildError = "", "git clone failed"
	if got := registryResponse(addon, contractBaseURL); !reflect.DeepEqual(got, want) {
		t.Errorf("after a failed rebuild = %+v, want %+v", got, want)
	}
}
