package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/auth"
	"github.com/wimwenigerkind/odoopack-registry/internal/middleware"
	"github.com/wimwenigerkind/odoopack-registry/internal/repository"
)

const bitbucketAPITimeout = 30 * time.Second

type IntegrationHandler struct {
	registry     *auth.Registry
	stateStore   *auth.StateStore
	integrations *repository.IntegrationRepository
	cookieSecure bool
	baseURL      string
}

func NewIntegrationHandler(reg *auth.Registry, stateStore *auth.StateStore, integrations *repository.IntegrationRepository, cookieSecure bool, baseURL string) *IntegrationHandler {
	return &IntegrationHandler{
		registry:     reg,
		stateStore:   stateStore,
		integrations: integrations,
		cookieSecure: cookieSecure,
		baseURL:      strings.TrimRight(baseURL, "/"),
	}
}

type integrationProviderSummary struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func (h *IntegrationHandler) ListProviders(c *gin.Context) {
	providers := h.registry.EnabledForIntegration()
	out := make([]integrationProviderSummary, 0, len(providers))
	for _, p := range providers {
		out = append(out, integrationProviderSummary{Name: p.Name(), Type: string(p.Type())})
	}
	c.JSON(http.StatusOK, gin.H{"providers": out})
}

func (h *IntegrationHandler) Connect(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	providerName := c.Param("provider")
	provider, err := h.registry.GetIntegration(providerName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown integration provider"})
		return
	}

	state := auth.NewState()
	if err := h.stateStore.Save(state, auth.FlowState{
		Provider: provider.Name(),
		Kind:     auth.FlowIntegration,
		UserID:   userID.String(),
		ReturnTo: sanitizeReturnTo(c.Query("return_to")),
	}, loginTTL); err != nil {
		internalError(c, "save state", err)
		return
	}

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(stateCookieName, state, int(loginTTL.Seconds()), statePathFor(provider.Name()), "", h.cookieSecure, true)
	c.Redirect(http.StatusFound, provider.IntegrationAuthURL(state))
}

func (h *IntegrationHandler) List(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	items, err := h.integrations.ListByOwner(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, items)
}

type createWorkspaceHookRequest struct {
	Workspace string `json:"workspace" binding:"required"`
}

func (h *IntegrationHandler) CreateWorkspaceHook(c *gin.Context) {
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
	it, err := h.integrations.GetByID(id)
	if errors.Is(err, repository.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "integration not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if it.OwnerID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "integration not owned by user"})
		return
	}
	if it.Provider != "bitbucket" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workspace hooks only supported for bitbucket"})
		return
	}

	var req createWorkspaceHookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	accessToken, err := auth.EnsureFreshToken(c.Request.Context(), h.registry, h.integrations, it)
	if err != nil {
		internalError(c, "refresh token", err)
		return
	}

	hookURL := h.baseURL + "/webhooks/bitbucket/" + it.ID.String()
	body := map[string]any{
		"description": "odoopack-registry sync",
		"url":         hookURL,
		"active":      true,
		"events":      []string{"repo:push"},
		"secret":      it.HookSecret,
	}
	payload, _ := json.Marshal(body)

	ctx, cancel := context.WithTimeout(c.Request.Context(), bitbucketAPITimeout)
	defer cancel()

	apiURL := "https://api.bitbucket.org/2.0/workspaces/" + url.PathEscape(req.Workspace) + "/hooks"
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(payload))
	httpReq.Header.Set("Authorization", "Bearer "+accessToken)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "bitbucket api unreachable"})
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated {
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "bitbucket rejected request: " + resp.Status,
			"details": strings.TrimSpace(string(respBody)),
		})
		return
	}

	var hookResp struct {
		UUID string `json:"uuid"`
	}
	_ = json.Unmarshal(respBody, &hookResp)
	c.JSON(http.StatusCreated, gin.H{
		"hook_uuid": hookResp.UUID,
		"workspace": req.Workspace,
	})
}

func (h *IntegrationHandler) Delete(c *gin.Context) {
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
	if err := h.integrations.Delete(id, userID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "integration not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.Status(http.StatusNoContent)
}
