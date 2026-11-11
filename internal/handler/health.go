package handler

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type HealthCheck struct {
	Name string
	Fn   func(context.Context) error
}

type HealthHandler struct {
	checks []HealthCheck
}

func NewHealthHandler(checks ...HealthCheck) *HealthHandler {
	return &HealthHandler{checks: checks}
}

func (h *HealthHandler) Live(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Ready answers with a bare status: /readyz is reachable through the ingress,
// and a check error can name hosts, users or database details.
func (h *HealthHandler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	ready := true
	for _, check := range h.checks {
		if err := check.Fn(ctx); err != nil {
			ready = false
			slog.Warn("readiness check failed", "check", check.Name, "err", err)
		}
	}

	if !ready {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
