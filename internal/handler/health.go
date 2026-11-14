package handler

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wimwenigerkind/odoopack-registry/internal/buildinfo"
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

// Live names the release so a running pod tells which one it is. Unlike the
// check errors Ready leaves out, the version of a public release is no secret.
func (h *HealthHandler) Live(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "version": buildinfo.Version})
}

// Ready answers with a bare status: the chart keeps /readyz out of the
// ingress, but another ingress may route it, and a check error can name hosts,
// users or database details.
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
