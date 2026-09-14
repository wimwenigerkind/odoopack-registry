package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/middleware"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"github.com/wimwenigerkind/odoopack-registry/internal/repository"
	"github.com/wimwenigerkind/odoopack-registry/internal/storage"
	"github.com/wimwenigerkind/odoopack-registry/internal/validate"
	paginate "github.com/wimwenigerkind/paginate"
	ginx "github.com/wimwenigerkind/paginate/ginx"
)

type AddonHandler struct {
	addons       *repository.AddonRepository
	repos        *repository.RepoRepository
	versions     *repository.AddonVersionRepository
	groups       *repository.GroupRepository
	users        *repository.UserRepository
	integrations *repository.IntegrationRepository
	storage      storage.Storage
	mode         string
}

func NewAddonHandler(addons *repository.AddonRepository, repos *repository.RepoRepository, versions *repository.AddonVersionRepository, groups *repository.GroupRepository, users *repository.UserRepository, integrations *repository.IntegrationRepository, store storage.Storage, mode string) *AddonHandler {
	return &AddonHandler{addons: addons, repos: repos, versions: versions, groups: groups, users: users, integrations: integrations, storage: store, mode: mode}
}

type registerAddonRequest struct {
	Name          string            `json:"name" binding:"required"`
	GitURL        string            `json:"git_url" binding:"required"`
	DefaultBranch string            `json:"default_branch"`
	Subpath       string            `json:"subpath"`
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
	if addon.Repo != nil {
		sanitizeOwner(addon.Repo.Owner)
	}
	annotateVersions(addon.Versions)
	if visible, err := h.addons.ListVisibleTo(currentUserIDPtr(c), isCurrentUserAdmin(c, h.users), ""); err == nil {
		allNames, _ := h.addons.ListAllNames()
		resolveDepends(addon.Versions, visible, allNames)
	}
	c.JSON(http.StatusOK, addon)
}

var addonPaginator = mustAddonPaginator()

func mustAddonPaginator() *paginate.Paginator {
	allowlist, err := paginate.NewAllowlist(paginate.AllowlistConfig{
		Fields: map[string][]paginate.Column{
			"name": {{Table: "addons", Name: "name"}},
		},
		Default:  "name",
		Tiebreak: paginate.Column{Table: "addons", Name: "id"},
	})
	if err != nil {
		panic(err)
	}
	p, err := paginate.New(paginate.Config{Allowlist: allowlist, DefaultLimit: 20, MaxLimit: 100})
	if err != nil {
		panic(err)
	}
	return p
}

func (h *AddonHandler) List(c *gin.Context) {
	q := h.addons.VisibleQuery(currentUserIDPtr(c), isCurrentUserAdmin(c, h.users), repository.AddonSearch{
		Query:  strings.TrimSpace(c.Query("q")),
		Series: strings.TrimSpace(c.Query("series")),
	}).
		Preload("Versions").
		Preload("Repo").
		Preload("Repo.Owner")

	page, err := paginate.Keyset[models.Addon](q, addonPaginator, ginx.Bind(c))
	if err != nil {
		if !ginx.Abort(c, err) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}
	for i := range page.Data {
		if page.Data[i].Repo != nil {
			sanitizeOwner(page.Data[i].Repo.Owner)
		}
		annotateVersions(page.Data[i].Versions)
	}
	c.JSON(http.StatusOK, page)
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
	if err := validate.GitURL(req.GitURL); err != nil {
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

	repo := &models.Repo{
		GitURL:        req.GitURL,
		DefaultBranch: defaultBranch,
		OwnerID:       userID,
		IntegrationID: req.IntegrationID,
	}
	created, err := h.repos.FindOrCreate(repo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if !created && repo.OwnerID != userID && !isCurrentUserAdmin(c, h.users) {
		c.JSON(http.StatusConflict, gin.H{"error": "addon could not be registered"})
		return
	}

	addon := &models.Addon{
		Name:       req.Name,
		RepoID:     repo.ID,
		Subpath:    req.Subpath,
		Visibility: visibility,
	}

	if err := h.addons.Create(addon); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	addon.Repo = repo
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

func (h *AddonHandler) Delete(c *gin.Context) {
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

	for _, v := range addon.Versions {
		if v.StorageKey == "" {
			continue
		}
		if err := h.storage.Delete(context.Background(), v.StorageKey); err != nil && !errors.Is(err, storage.ErrNotFound) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not remove zipball"})
			return
		}
	}

	if err := h.addons.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.Status(http.StatusNoContent)
}

type updateAddonRequest struct {
	Name       string            `json:"name"`
	Subpath    string            `json:"subpath"`
	Visibility models.Visibility `json:"visibility"`
}

func (h *AddonHandler) Update(c *gin.Context) {
	if _, ok := middleware.CurrentUserID(c); !ok {
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

	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	addon.Name = name
	addon.Subpath = req.Subpath
	if req.Visibility != "" {
		addon.Visibility = req.Visibility
	}

	if err := h.addons.Update(addon); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "addon name already taken"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, addon)
}

func (h *AddonHandler) Readme(c *gin.Context) {
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

	readme, err := h.versions.GetReadme(av.ID)
	if errors.Is(err, repository.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "no readme for this version"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, readme)
}

func generateSecret(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
