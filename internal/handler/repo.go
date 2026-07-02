package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/middleware"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"github.com/wimwenigerkind/odoopack-registry/internal/repository"
)

type RepoHandler struct {
	repos        *repository.RepoRepository
	addons       *repository.AddonRepository
	groups       *repository.GroupRepository
	users        *repository.UserRepository
	integrations *repository.IntegrationRepository
	mode         string
}

func NewRepoHandler(repos *repository.RepoRepository, addons *repository.AddonRepository, groups *repository.GroupRepository, users *repository.UserRepository, integrations *repository.IntegrationRepository, mode string) *RepoHandler {
	return &RepoHandler{repos: repos, addons: addons, groups: groups, users: users, integrations: integrations, mode: mode}
}

func (h *RepoHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	repo, err := h.repos.GetByID(id)
	if errors.Is(err, repository.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if !canReadRepo(c, repo, repo.Addons, h.groups, h.users, h.mode) {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
		return
	}

	readable := make([]models.Addon, 0, len(repo.Addons))
	for i := range repo.Addons {
		addon := repo.Addons[i]
		addon.Repo = repo
		if canReadAddon(c, &addon, h.groups, h.users, h.mode) {
			readable = append(readable, repo.Addons[i])
		}
	}
	repo.Addons = readable

	if !canWriteRepo(c, repo, h.users) {
		repo.IntegrationID = nil
		repo.Integration = nil
	}
	sanitizeOwner(repo.Owner)

	c.JSON(http.StatusOK, repo)
}

func (h *RepoHandler) ListMine(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	repos, err := h.repos.ListByOwner(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, repos)
}

type updateRepoRequest struct {
	DefaultBranch *string         `json:"default_branch"`
	IntegrationID json.RawMessage `json:"integration_id"`
}

func (h *RepoHandler) Update(c *gin.Context) {
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
	repo, err := h.repos.GetByID(id)
	if errors.Is(err, repository.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if !canWriteRepo(c, repo, h.users) {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
		return
	}

	var req updateRepoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.DefaultBranch != nil {
		branch := *req.DefaultBranch
		if branch == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "default_branch cannot be empty"})
			return
		}
		repo.DefaultBranch = branch
	}

	if len(req.IntegrationID) > 0 {
		if bytes.Equal(bytes.TrimSpace(req.IntegrationID), []byte("null")) {
			repo.IntegrationID = nil
		} else {
			var idStr string
			if err := json.Unmarshal(req.IntegrationID, &idStr); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid integration_id"})
				return
			}
			integrationID, err := uuid.Parse(idStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid integration_id"})
				return
			}
			it, err := h.integrations.GetByID(integrationID)
			if err != nil || it == nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "integration not found"})
				return
			}
			if it.OwnerID != userID {
				c.JSON(http.StatusForbidden, gin.H{"error": "integration not owned by user"})
				return
			}
			repo.IntegrationID = &integrationID
		}
	}

	if err := h.repos.Update(repo); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	sanitizeOwner(repo.Owner)
	c.JSON(http.StatusOK, repo)
}

func (h *RepoHandler) Delete(c *gin.Context) {
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
	repo, err := h.repos.GetByID(id)
	if errors.Is(err, repository.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if !canWriteRepo(c, repo, h.users) {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
		return
	}

	count, err := h.repos.CountAddons(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "repo still has addons; delete them first"})
		return
	}
	if err := h.repos.Delete(id, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.Status(http.StatusNoContent)
}
