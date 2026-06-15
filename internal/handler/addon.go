package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/middleware"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"github.com/wimwenigerkind/odoopack-registry/internal/repository"
	"github.com/wimwenigerkind/odoopack-registry/internal/storage"
)

type AddonHandler struct {
	addons       *repository.AddonRepository
	versions     *repository.AddonVersionRepository
	groups       *repository.GroupRepository
	users        *repository.UserRepository
	integrations *repository.IntegrationRepository
	storage      storage.Storage
	mode         string
}

func NewAddonHandler(addons *repository.AddonRepository, versions *repository.AddonVersionRepository, groups *repository.GroupRepository, users *repository.UserRepository, integrations *repository.IntegrationRepository, store storage.Storage, mode string) *AddonHandler {
	return &AddonHandler{addons: addons, versions: versions, groups: groups, users: users, integrations: integrations, storage: store, mode: mode}
}

type registerAddonRequest struct {
	Name          string            `json:"name" binding:"required"`
	GitURL        string            `json:"git_url" binding:"required"`
	DefaultBranch string            `json:"default_branch"`
	Visibility    models.Visibility `json:"visibility"`
	IntegrationID *uuid.UUID        `json:"integration_id,omitempty"`
}

func (h *AddonHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
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
	c.JSON(http.StatusOK, addon)
}

func (h *AddonHandler) List(c *gin.Context) {
	addons, err := h.addons.ListVisibleTo(currentUserIDPtr(c), isCurrentUserAdmin(c, h.users), c.Query("name"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, addons)
}

func (h *AddonHandler) Register(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	var req registerAddonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	defaultBranch := req.DefaultBranch
	if defaultBranch == "" {
		defaultBranch = "main"
	}
	visibility := req.Visibility
	if visibility == "" {
		visibility = models.VisibilityPublic
	}

	if req.IntegrationID != nil {
		it, err := h.integrations.GetByID(*req.IntegrationID)
		if err != nil || it == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "integration not found"})
			return
		}
		if it.OwnerID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "integration not owned by user"})
			return
		}
	}

	addon := &models.Addon{
		Name:          req.Name,
		GitURL:        req.GitURL,
		DefaultBranch: defaultBranch,
		Visibility:    visibility,
		OwnerID:       userID,
		IntegrationID: req.IntegrationID,
	}

	if err := h.addons.Create(addon); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusCreated, addon)
}

func (h *AddonHandler) DeleteVersion(c *gin.Context) {
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
	if !canWriteAddon(c, addon, h.users) {
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

	if av.StorageKey != "" {
		if err := h.storage.Delete(context.Background(), av.StorageKey); err != nil && !errors.Is(err, storage.ErrNotFound) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not remove zipball"})
			return
		}
	}
	if err := h.versions.Delete(av.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.Status(http.StatusNoContent)
}

type updateAddonRequest struct {
	GitURL        string            `json:"git_url" binding:"required"`
	DefaultBranch string            `json:"default_branch"`
	Visibility    models.Visibility `json:"visibility"`
	IntegrationID *uuid.UUID        `json:"integration_id"`
}

func (h *AddonHandler) Update(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
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
	if !canWriteAddon(c, addon, h.users) {
		c.JSON(http.StatusNotFound, gin.H{"error": "addon not found"})
		return
	}

	var req updateAddonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.IntegrationID != nil {
		it, err := h.integrations.GetByID(*req.IntegrationID)
		if err != nil || it == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "integration not found"})
			return
		}
		if it.OwnerID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "integration not owned by user"})
			return
		}
	}

	addon.GitURL = req.GitURL
	if req.DefaultBranch != "" {
		addon.DefaultBranch = req.DefaultBranch
	}
	if req.Visibility != "" {
		addon.Visibility = req.Visibility
	}
	addon.IntegrationID = req.IntegrationID

	if err := h.addons.Update(addon); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, addon)
}

func generateSecret(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
