package handler

import (
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"github.com/wimwenigerkind/odoopack-spec/version"
)

func annotateVersions(versions []models.AddonVersion) {
	latestIdx := -1
	var latest version.Version
	for i := range versions {
		v, err := version.Parse(versions[i].Version)
		if err != nil {
			continue
		}
		versions[i].Series = v.SeriesString()
		if v.IsRelease() && versions[i].Status == models.StatusReady {
			if latestIdx == -1 || version.Compare(v, latest) > 0 {
				latestIdx = i
				latest = v
			}
		}
	}
	if latestIdx >= 0 {
		versions[latestIdx].IsLatest = true
	}
}
