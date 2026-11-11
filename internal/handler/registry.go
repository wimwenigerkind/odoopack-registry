package handler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/auth"
	"github.com/wimwenigerkind/odoopack-registry/internal/git"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"github.com/wimwenigerkind/odoopack-registry/internal/repository"
	"github.com/wimwenigerkind/odoopack-registry/internal/storage"
	"github.com/wimwenigerkind/odoopack-spec/digest"
	"github.com/wimwenigerkind/odoopack-spec/registryapi"
)

type RegistryHandler struct {
	addons       *repository.AddonRepository
	groups       *repository.GroupRepository
	users        *repository.UserRepository
	integrations *repository.IntegrationRepository
	authRegistry *auth.Registry
	storage      storage.Storage
	mode         string
	baseURL      string
}

func NewRegistryHandler(addons *repository.AddonRepository, groups *repository.GroupRepository, users *repository.UserRepository, integrations *repository.IntegrationRepository, authRegistry *auth.Registry, store storage.Storage, mode, baseURL string) *RegistryHandler {
	return &RegistryHandler{
		addons:       addons,
		groups:       groups,
		users:        users,
		integrations: integrations,
		authRegistry: authRegistry,
		storage:      store,
		mode:         mode,
		baseURL:      strings.TrimRight(baseURL, "/"),
	}
}

var referenceRe = regexp.MustCompile(`^[0-9a-fA-F]{7,64}$`)

func (h *RegistryHandler) Get(c *gin.Context) {
	name := strings.TrimPrefix(c.Param("name"), "/")
	addon, err := h.addons.GetByName(name)
	if errors.Is(err, repository.ErrNotFound) {
		problem(c, http.StatusNotFound, codeNotFound, "addon not found")
		return
	}
	if err != nil {
		problem(c, http.StatusInternalServerError, codeInternalError, "internal error")
		return
	}
	if !canReadAddon(c, addon, h.groups, h.users, h.mode) {
		problem(c, http.StatusNotFound, codeNotFound, "addon not found")
		return
	}

	if visible, err := h.addons.ListVisibleTo(currentUserIDPtr(c), isCurrentUserAdmin(c, h.users), h.mode, ""); err == nil {
		allNames, _ := h.addons.ListAllNames()
		resolveDepends(addon.Versions, visible, allNames)
	}

	c.JSON(http.StatusOK, registryResponse(addon, h.baseURL))
}

// registryResponse lists only the ready versions, with the depends that
// resolveDepends filled in; it touches no database so the contract test can
// check its output against docs/openapi.yaml. Rows published before the
// worker enforced canonical versions are still in the database; they are
// left out instead of breaking the contract.
func registryResponse(addon *models.Addon, baseURL string) registryapi.Addon {
	versions := make([]registryapi.Version, 0, len(addon.Versions))
	for _, v := range addon.Versions {
		if v.Status != models.StatusReady {
			continue
		}
		sum, err := digest.Parse(v.ContentHash)
		if err != nil {
			slog.Warn("skipping version with invalid content hash", "addon", addon.Name, "version", v.Version, "err", err)
			continue
		}
		deps := make([]registryapi.Dep, 0, len(v.DependsResolved))
		for _, rd := range v.DependsResolved {
			deps = append(deps, registryapi.Dep{Module: rd.Module, Package: rd.Name, Access: rd.Access})
		}
		rv := registryapi.Version{
			Version:   v.Version,
			Type:      registryapi.TypeZip,
			URL:       fmt.Sprintf("%s/registry/v1/zipball/%s/%s", baseURL, addon.ID, sum.Hex()),
			Shasum:    sum.String(),
			Reference: v.RefValue,
			Depends:   deps,
		}
		if err := rv.Validate(); err != nil {
			slog.Warn("skipping version outside the registry contract", "addon", addon.Name, "version", v.Version, "err", err)
			continue
		}
		versions = append(versions, rv)
	}
	return registryapi.Addon{Name: addon.Name, Versions: versions}
}

func (h *RegistryHandler) Zipball(c *gin.Context) {
	id, err := uuid.Parse(c.Param("addon_id"))
	if err != nil {
		problem(c, http.StatusBadRequest, codeMalformedRequest, "invalid addon id")
		return
	}
	reference := strings.ToLower(c.Param("reference"))
	if !referenceRe.MatchString(reference) {
		problem(c, http.StatusBadRequest, codeMalformedRequest, "invalid reference")
		return
	}
	addon, err := h.addons.GetByID(id)
	if errors.Is(err, repository.ErrNotFound) {
		problem(c, http.StatusNotFound, codeNotFound, "addon not found")
		return
	}
	if err != nil {
		problem(c, http.StatusInternalServerError, codeInternalError, "internal error")
		return
	}
	if !canReadAddon(c, addon, h.groups, h.users, h.mode) {
		problem(c, http.StatusNotFound, codeNotFound, "addon not found")
		return
	}

	// Only a full sha256 hex is a content hash; a shorter reference is a commit.
	if want, err := digest.Parse("sha256:" + reference); err == nil {
		for i := range addon.Versions {
			av := &addon.Versions[i]
			if av.Status != models.StatusReady {
				continue
			}
			if got, err := digest.Parse(av.ContentHash); err != nil || got != want {
				continue
			}
			if rc, err := h.storage.Get(context.Background(), av.StorageKey); err == nil {
				streamZip(c, rc, addon.Name, av.Version, av.RefValue)
				return
			}
			problem(c, http.StatusNotFound, codeNotFound, "artifact unavailable")
			return
		}
	}

	for i := range addon.Versions {
		if addon.Versions[i].RefValue == reference {
			av := &addon.Versions[i]
			if av.Status != models.StatusReady {
				problem(c, http.StatusConflict, codeConflict, fmt.Sprintf("version not ready: %s", av.Status))
				return
			}
			if rc, err := h.storage.Get(context.Background(), av.StorageKey); err == nil {
				streamZip(c, rc, addon.Name, av.Version, reference)
				return
			}
		}
	}

	// TODO: store build packages in db.
	// Until then, historical builds live only in storage, keyed by <version>-<sha>.zip.
	// We probe every known version name of this addon in case one of them was built at
	// this SHA at some point.
	for _, v := range addon.Versions {
		key := packageStorageKey(addon.ID, v.Version, reference)
		if rc, err := h.storage.Get(context.Background(), key); err == nil {
			streamZip(c, rc, addon.Name, v.Version, reference)
			return
		}
	}

	pinnedKey := pinnedStorageKey(addon.ID, reference)
	if rc, err := h.storage.Get(context.Background(), pinnedKey); err == nil {
		streamZip(c, rc, addon.Name, "pinned", reference)
		return
	}

	if !canWriteAddon(c, addon, h.users) {
		problem(c, http.StatusNotFound, codeNotFound, "reference not found")
		return
	}
	if addon.Repo == nil {
		problem(c, http.StatusNotFound, codeNotFound, "reference not found")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Minute)
	defer cancel()

	remote, err := auth.ResolveGitRemote(ctx, h.authRegistry, h.integrations, addon.Repo.GitURL, addon.Repo.IntegrationID)
	if err != nil {
		slog.Warn("on-demand rebuild: resolve git remote", "sha", reference, "err", err)
		problem(c, http.StatusNotFound, codeNotFound, "reference not found")
		return
	}

	rootDir := rootDirFor(addon.Name, addon.Subpath)
	archive, err := git.CloneAndZipAtSHA(ctx, remote.URL, remote.Env, reference, rootDir, addon.Subpath)
	if err != nil {
		slog.Warn("on-demand rebuild failed", "sha", reference, "err", auth.RedactURLCredentials(err.Error()))
		problem(c, http.StatusNotFound, codeNotFound, "reference not found in git")
		return
	}
	defer func() { _ = os.Remove(archive.ZipPath) }()

	gotHash := "sha256:" + archive.ContentHash
	if expected := expectedContentHash(addon.Versions, reference); expected != "" && !sameDigest(expected, gotHash) {
		slog.Warn("on-demand rebuild hash mismatch, refusing to serve",
			"sha", reference, "expected", expected, "got", gotHash)
		problem(c, http.StatusConflict, codeConflict, "artifact unavailable: rebuilt content does not match the published checksum")
		return
	}

	if f, err := os.Open(archive.ZipPath); err == nil {
		_ = h.storage.Put(ctx, pinnedKey, f, storage.PutOptions{ContentType: "application/zip"})
		_ = f.Close()
	}

	f, err := os.Open(archive.ZipPath)
	if err != nil {
		problem(c, http.StatusInternalServerError, codeInternalError, "internal error")
		return
	}
	streamZip(c, f, addon.Name, "pinned", reference)
}

func expectedContentHash(versions []models.AddonVersion, reference string) string {
	for i := range versions {
		v := &versions[i]
		if v.RefValue == reference {
			return v.ContentHash
		}
	}
	return ""
}

// sameDigest treats a value that does not parse as a mismatch, so a corrupt
// stored hash refuses the rebuild instead of serving unverified content.
func sameDigest(a, b string) bool {
	da, errA := digest.Parse(a)
	db, errB := digest.Parse(b)
	return errA == nil && errB == nil && da == db
}

func packageStorageKey(addonID uuid.UUID, version, sha string) string {
	key := strings.ReplaceAll(version+"-"+sha, "/", "-")
	return fmt.Sprintf("packages/%s/%s.zip", addonID, key)
}

func pinnedStorageKey(addonID uuid.UUID, sha string) string {
	return fmt.Sprintf("packages/%s/pinned-%s.zip", addonID, sha)
}

func rootDirFor(name, subpath string) string {
	if cleaned := strings.Trim(subpath, "/"); cleaned != "" {
		return path.Base(cleaned)
	}
	if i := strings.LastIndex(name, "/"); i >= 0 {
		return name[i+1:]
	}
	return name
}

func streamZip(c *gin.Context, rc io.ReadCloser, name, version, reference string) {
	defer func() { _ = rc.Close() }()
	short := reference
	if len(short) > 12 {
		short = short[:12]
	}
	filename := fmt.Sprintf("%s-%s-%s.zip",
		strings.ReplaceAll(name, "/", "_"),
		strings.ReplaceAll(version, "/", "-"),
		short,
	)
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	_, _ = io.Copy(c.Writer, rc)
}
