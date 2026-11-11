package handler

import (
	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"github.com/wimwenigerkind/odoopack-registry/internal/repository"
	"github.com/wimwenigerkind/odoopack-spec/name"
	"github.com/wimwenigerkind/odoopack-spec/registryapi"
)

// resolveVersionDepends loads only the addons that provide a module the
// versions depend on, so its cost follows the manifests and not the size of
// the registry. A failed visibility lookup leaves the depends unresolved.
func resolveVersionDepends(addons *repository.AddonRepository, versions []models.AddonVersion, userID *uuid.UUID, isAdmin bool, mode string) {
	modules := dependedModules(versions)
	if len(modules) == 0 {
		return
	}
	visible, err := addons.ListVisibleByModule(userID, isAdmin, mode, modules)
	if err != nil {
		return
	}
	existing, _ := addons.ListNamesByModule(modules)
	resolveDepends(versions, visible, existing)
}

func dependedModules(versions []models.AddonVersion) []string {
	seen := make(map[string]bool)
	var modules []string
	for _, v := range versions {
		for _, module := range v.Depends {
			if !seen[module] {
				seen[module] = true
				modules = append(modules, module)
			}
		}
	}
	return modules
}

func resolveDepends(versions []models.AddonVersion, visible []models.Addon, allNames []string) {
	visByModule := make(map[string]*models.Addon, len(visible))
	for i := range visible {
		visByModule[name.ModuleName(visible[i].Name)] = &visible[i]
	}
	existsByModule := make(map[string]bool, len(allNames))
	for _, n := range allNames {
		existsByModule[name.ModuleName(n)] = true
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
				rd.Access = registryapi.AccessOK
			case existsByModule[module]:
				rd.Access = registryapi.AccessForbidden
			default:
				rd.Access = registryapi.AccessExternal
			}
			resolved = append(resolved, rd)
		}
		versions[vi].DependsResolved = resolved
	}
}
