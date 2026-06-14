package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/auth"
	"github.com/wimwenigerkind/odoopack-registry/internal/middleware"
	"github.com/wimwenigerkind/odoopack-registry/internal/repository"
)

type IntegrationHandler struct {
	registry     *auth.Registry
	stateStore   *auth.StateStore
	integrations *repository.IntegrationRepository
	cookieSecure bool
}

func NewIntegrationHandler(reg *auth.Registry, stateStore *auth.StateStore, integrations *repository.IntegrationRepository, cookieSecure bool) *IntegrationHandler {
	return &IntegrationHandler{
		registry:     reg,
		stateStore:   stateStore,
		integrations: integrations,
		cookieSecure: cookieSecure,
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
