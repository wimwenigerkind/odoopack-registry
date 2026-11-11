package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestReadyHidesCheckErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHealthHandler(
		HealthCheck{Name: "database", Fn: func(context.Context) error { return nil }},
		HealthCheck{Name: "redis", Fn: func(context.Context) error {
			return errors.New("dial tcp 10.0.0.7:6379: connection refused")
		}},
	)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/readyz", nil)
	h.Ready(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
	if body := strings.TrimSpace(w.Body.String()); body != `{"status":"unavailable"}` {
		t.Errorf("body = %s, want only the status", body)
	}
}

func TestReadyOK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHealthHandler(HealthCheck{Name: "database", Fn: func(context.Context) error { return nil }})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/readyz", nil)
	h.Ready(c)

	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != `{"status":"ok"}` {
		t.Errorf("got %d %s, want 200 {\"status\":\"ok\"}", w.Code, w.Body.String())
	}
}
