package handler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"github.com/wimwenigerkind/odoopack-registry/internal/repository"
	"github.com/wimwenigerkind/odoopack-registry/internal/storage"
)

type RegistryHandler struct {
	addons   *repository.AddonRepository
	versions *repository.AddonVersionRepository
	groups   *repository.GroupRepository
	users    *repository.UserRepository
	storage  storage.Storage
	mode     string
	baseURL  string
}

func NewRegistryHandler(addons *repository.AddonRepository, versions *repository.AddonVersionRepository, groups *repository.GroupRepository, users *repository.UserRepository, store storage.Storage, mode, baseURL string) *RegistryHandler {
	return &RegistryHandler{
		addons:   addons,
		versions: versions,
		groups:   groups,
		users:    users,
		storage:  store,
		mode:     mode,
		baseURL:  strings.TrimRight(baseURL, "/"),
	}
}

type registryVersion struct {
	Version string `json:"version"`
	Type    string `json:"type"`
	URL     string `json:"url"`
	Shasum  string `json:"shasum,omitempty"`
	Ref     string `json:"ref,omitempty"`
}

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
			Version: v.Version,
			Type:    "zip",
			URL:     fmt.Sprintf("%s/registry/v1/zipball/%s/%s", h.baseURL, addon.ID, v.Version),
			Shasum:  v.ContentHash,
			Ref:     v.RefValue,
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
	version := c.Param("version")

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

	av, err := h.versions.Get(addon.ID, version)
	if errors.Is(err, repository.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "version not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if av.Status != models.StatusReady {
		c.JSON(http.StatusConflict, gin.H{"error": "version not ready", "status": av.Status})
		return
	}

	rc, err := h.storage.Get(context.Background(), av.StorageKey)
	if errors.Is(err, storage.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "zipball missing from storage"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	defer rc.Close()

	filename := strings.ReplaceAll(addon.Name, "/", "_") + "-" + strings.ReplaceAll(version, "/", "_") + ".zip"
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	_, _ = io.Copy(c.Writer, rc)
}
