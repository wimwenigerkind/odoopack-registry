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

type registryVersion struct {
	Version   string `json:"version"`
	Type      string `json:"type"`
	URL       string `json:"url"`
	Shasum    string `json:"shasum,omitempty"`
	Reference string `json:"reference,omitempty"`
}

var referenceRe = regexp.MustCompile(`^[0-9a-fA-F]{7,64}$`)

type registryAddon struct {
	Name     string            `json:"name"`
	Versions []registryVersion `json:"versions"`
}

func (h *RegistryHandler) Get(c *gin.Context) {
	name := strings.TrimPrefix(c.Param("name"), "/")
	addon, err := h.addons.GetByName(name)
	if errors.Is(err, repository.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "addon not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if !canReadAddon(c, addon, h.groups, h.users, h.mode) {
		c.JSON(http.StatusNotFound, gin.H{"error": "addon not found"})
		return
	}

	versions := make([]registryVersion, 0, len(addon.Versions))
	for _, v := range addon.Versions {
		if v.Status != models.StatusReady {
			continue
		}
		versions = append(versions, registryVersion{
			Version:   v.Version,
			Type:      "zip",
			URL:       fmt.Sprintf("%s/registry/v1/zipball/%s/%s", h.baseURL, addon.ID, strings.TrimPrefix(v.ContentHash, "sha256:")),
			Shasum:    v.ContentHash,
			Reference: v.RefValue,
		})
	}

	c.JSON(http.StatusOK, registryAddon{Name: addon.Name, Versions: versions})
}

func (h *RegistryHandler) Zipball(c *gin.Context) {
	id, err := uuid.Parse(c.Param("addon_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid addon id"})
		return
	}
	reference := strings.ToLower(c.Param("reference"))
	if !referenceRe.MatchString(reference) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid reference"})
		return
	}
	addon, err := h.addons.GetByID(id)
	if errors.Is(err, repository.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "addon not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if !canReadAddon(c, addon, h.groups, h.users, h.mode) {
		c.JSON(http.StatusNotFound, gin.H{"error": "addon not found"})
		return
	}

	wantHash := "sha256:" + reference
	for i := range addon.Versions {
		av := &addon.Versions[i]
		if av.Status == models.StatusReady && av.ContentHash == wantHash {
			if rc, err := h.storage.Get(context.Background(), av.StorageKey); err == nil {
				streamZip(c, rc, addon.Name, av.Version, av.RefValue)
				return
			}
			c.JSON(http.StatusNotFound, gin.H{"error": "artifact unavailable"})
			return
		}
	}

	for i := range addon.Versions {
		if addon.Versions[i].RefValue == reference {
			av := &addon.Versions[i]
			if av.Status != models.StatusReady {
				c.JSON(http.StatusConflict, gin.H{"error": "version not ready", "status": av.Status})
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
		c.JSON(http.StatusNotFound, gin.H{"error": "reference not found"})
		return
	}
	if addon.Repo == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "reference not found"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Minute)
	defer cancel()

	cloneURL, err := auth.ResolveCloneURL(ctx, h.authRegistry, h.integrations, addon.Repo.GitURL, addon.Repo.IntegrationID)
	if err != nil {
		slog.Warn("on-demand rebuild: resolve clone url", "sha", reference, "err", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "reference not found"})
		return
	}

	rootDir := rootDirFor(addon.Name, addon.Subpath)
	archive, err := git.CloneAndZipAtSHA(cloneURL, reference, rootDir, addon.Subpath)
	if err != nil {
		slog.Warn("on-demand rebuild failed", "sha", reference, "err", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "reference not found in git"})
		return
	}
	defer os.Remove(archive.ZipPath)

	gotHash := "sha256:" + archive.ContentHash
	if expected := expectedContentHash(addon.Versions, reference); expected != "" && gotHash != expected {
		slog.Warn("on-demand rebuild hash mismatch, refusing to serve",
			"sha", reference, "expected", expected, "got", gotHash)
		c.JSON(http.StatusConflict, gin.H{"error": "artifact unavailable: rebuilt content does not match the published checksum"})
		return
	}

	if f, err := os.Open(archive.ZipPath); err == nil {
		_ = h.storage.Put(ctx, pinnedKey, f, storage.PutOptions{ContentType: "application/zip"})
		f.Close()
	}

	f, err := os.Open(archive.ZipPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
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
	defer rc.Close()
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
