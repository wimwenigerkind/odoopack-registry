package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"github.com/wimwenigerkind/odoopack-registry/internal/repository"
	"github.com/wimwenigerkind/odoopack-registry/internal/storage"
)

type DownloadHandler struct {
	addons   *repository.AddonRepository
	versions *repository.AddonVersionRepository
	storage  storage.Storage
}

func NewDownloadHandler(addons *repository.AddonRepository, versions *repository.AddonVersionRepository, store storage.Storage) *DownloadHandler {
	return &DownloadHandler{addons: addons, versions: versions, storage: store}
}

func (h *DownloadHandler) Zipball(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
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
	if !canReadAddon(c, addon) {
		c.JSON(http.StatusNotFound, gin.H{"error": "addon not found"})
		return
	}

	av, err := h.versions.Get(addon.ID, version)
	if errors.Is(err, repository.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "version not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rc.Close()

	filename := strings.ReplaceAll(addon.Name, "/", "_") + "-" + strings.ReplaceAll(version, "/", "_") + ".zip"
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	_, _ = io.Copy(c.Writer, rc)
}
