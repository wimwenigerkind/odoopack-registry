package handler

import (
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"github.com/wimwenigerkind/odoopack-registry/internal/naming"
)

func resolveDepends(versions []models.AddonVersion, visible []models.Addon, allNames []string) {
	visByModule := make(map[string]*models.Addon, len(visible))
	for i := range visible {
		visByModule[naming.ModuleName(visible[i].Name)] = &visible[i]
	}
	existsByModule := make(map[string]bool, len(allNames))
	for _, n := range allNames {
		existsByModule[naming.ModuleName(n)] = true
	}

	for vi := range versions {
		if len(versions[vi].Depends) == 0 {
			continue
		}
		resolved := make([]models.ResolvedDep, 0, len(versions[vi].Depends))
		for _, module := range versions[vi].Depends {
			rd := models.ResolvedDep{Module: module}
			switch {
			case visByModule[module] != nil:
				a := visByModule[module]
				id := a.ID
				rd.AddonID = &id
				rd.Name = a.Name
				rd.Access = "ok"
			case existsByModule[module]:
				rd.Access = "forbidden"
			default:
				rd.Access = "external"
			}
			resolved = append(resolved, rd)
		}
		versions[vi].DependsResolved = resolved
	}
}
