package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"github.com/wimwenigerkind/odoopack-registry/internal/repository"
	"github.com/wimwenigerkind/odoopack-registry/internal/storage"
)

type RegistryHandler struct {
	addons  *repository.AddonRepository
	groups  *repository.GroupRepository
	users   *repository.UserRepository
	tokens  *repository.ApiTokenRepository
	storage storage.Storage
	mode    string
}

func NewRegistryHandler(addons *repository.AddonRepository, groups *repository.GroupRepository, users *repository.UserRepository, tokens *repository.ApiTokenRepository, store storage.Storage, mode string) *RegistryHandler {
	return &RegistryHandler{addons: addons, groups: groups, users: users, tokens: tokens, storage: store, mode: mode}
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

	if !h.canRead(c, addon) {
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

func (h *RegistryHandler) canRead(c *gin.Context, addon *models.Addon) bool {
	if addon.Visibility == models.VisibilityPublic && h.mode != "private" {
		return true
	}
	tok, ok := h.resolveBearer(c)
	if !ok {
		return false
	}
	user, err := h.users.GetByID(tok.UserID)
	if err != nil || user == nil {
		return false
	}
	if addon.Visibility == models.VisibilityPublic {
		return true
	}
	if user.IsAdmin {
		return true
	}
	if addon.OwnerID == tok.UserID {
		return true
	}
	can, err := h.groups.UserCanReadAddon(tok.UserID, addon.ID)
	if err != nil {
		return false
	}
	return can
}

func (h *RegistryHandler) resolveBearer(c *gin.Context) (*models.ApiToken, bool) {
	header := c.GetHeader("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return nil, false
	}
	plain := strings.TrimSpace(header[len(prefix):])
	if plain == "" {
		return nil, false
	}
	tok, err := h.tokens.GetByToken(plain)
	if err != nil || tok == nil {
		return nil, false
	}
	_ = h.tokens.TouchLastUsed(tok.ID, time.Now())
	return tok, true
}
