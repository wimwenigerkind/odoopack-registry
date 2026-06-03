package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"github.com/wimwenigerkind/odoopack-registry/internal/repository"
	"github.com/wimwenigerkind/odoopack-registry/internal/storage"
)

type RegistryHandler struct {
	addons  *repository.AddonRepository
	storage storage.Storage
}

func NewRegistryHandler(addons *repository.AddonRepository, store storage.Storage) *RegistryHandler {
	return &RegistryHandler{addons: addons, storage: store}
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

	if addon.Visibility != models.VisibilityPublic {
		c.JSON(http.StatusNotFound, gin.H{"error": "addon not found"})
		return
	}

	versions := make([]registryVersion, 0, len(addon.Versions))
	for _, v := range addon.Versions {
		if v.Status != models.StatusReady {
			continue
		}
		url, err := h.storage.URL(context.Background(), v.StorageKey, storage.URLOptions{})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		versions = append(versions, registryVersion{
			Version: v.Version,
			Type:    "zip",
			URL:     url,
			Shasum:  v.ContentHash,
			Ref:     v.RefValue,
		})
	}

	c.JSON(http.StatusOK, registryAddon{Name: addon.Name, Versions: versions})
}
