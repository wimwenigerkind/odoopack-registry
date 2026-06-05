package handler

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/middleware"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"github.com/wimwenigerkind/odoopack-registry/internal/repository"
)

type AddonHandler struct {
	addons *repository.AddonRepository
	groups *repository.GroupRepository
	users  *repository.UserRepository
}

func NewAddonHandler(addons *repository.AddonRepository, groups *repository.GroupRepository, users *repository.UserRepository) *AddonHandler {
	return &AddonHandler{addons: addons, groups: groups, users: users}
}

type registerAddonRequest struct {
	Name          string             `json:"name" binding:"required"`
	GitProvider   models.GitProvider `json:"git_provider"`
	GitURL        string             `json:"git_url" binding:"required"`
	DefaultBranch string             `json:"default_branch"`
	Visibility    models.Visibility  `json:"visibility"`
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
	if !canReadAddon(c, addon, h.groups, h.users) {
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

	secret, err := generateSecret(32)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
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
	provider := req.GitProvider
	if provider == "" {
		provider = models.ProviderGeneric
	}

	addon := &models.Addon{
		Name:          req.Name,
		GitProvider:   provider,
		GitURL:        req.GitURL,
		DefaultBranch: defaultBranch,
		Visibility:    visibility,
		WebhookSecret: secret,
		OwnerID:       userID,
	}

	if err := h.addons.Create(addon); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"addon":          addon,
		"webhook_secret": secret,
	})
}

func generateSecret(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
