package handler

import (
	"context"
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

func (h *HealthHandler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	results := make(map[string]string, len(h.checks))
	ready := true
	for _, check := range h.checks {
		if err := check.Fn(ctx); err != nil {
			ready = false
			results[check.Name] = "error: " + err.Error()
		} else {
			results[check.Name] = "ok"
		}
	}

	status := http.StatusOK
	overall := "ok"
	if !ready {
		status = http.StatusServiceUnavailable
		overall = "unavailable"
	}
	c.JSON(status, gin.H{"status": overall, "checks": results})
}
