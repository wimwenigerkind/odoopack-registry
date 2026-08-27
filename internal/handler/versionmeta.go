package handler

import (
	odoosemver "github.com/wimwenigerkind/odoopack-semver"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
)

func annotateVersions(versions []models.AddonVersion) {
	latestIdx := -1
	var latest odoosemver.Version
	for i := range versions {
		v, err := odoosemver.Parse(versions[i].Version)
		if err != nil {
			continue
		}
		versions[i].Series = v.SeriesString()
		if v.IsRelease() && versions[i].Status == models.StatusReady {
			if latestIdx == -1 || odoosemver.Compare(v, latest) > 0 {
				latestIdx = i
				latest = v
			}
		}
	}
	if latestIdx >= 0 {
		versions[latestIdx].IsLatest = true
	}
}
