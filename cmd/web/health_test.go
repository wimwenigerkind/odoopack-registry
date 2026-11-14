package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/auth"
	"github.com/wimwenigerkind/odoopack-registry/internal/middleware"
)

type downSessions struct{}

func (downSessions) Create(uuid.UUID, time.Duration, string, string) (auth.Session, error) {
	return auth.Session{}, fmt.Errorf("%w: connection refused", auth.ErrStoreUnavailable)
}

func (downSessions) Get(string) (auth.Session, error) {
	return auth.Session{}, fmt.Errorf("%w: connection refused", auth.ErrStoreUnavailable)
}

func (downSessions) Delete(string) error {
	return fmt.Errorf("%w: connection refused", auth.ErrStoreUnavailable)
}

func (downSessions) Stop() {}

func TestReadyWithoutSessionStore(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/me", middleware.RequireAuth(downSessions{}), func(c *gin.Context) { c.Status(http.StatusOK) })
	registerHealth(r, func(context.Context) error { return nil })

	for _, tc := range []struct {
		path string
		want int
	}{
		{"/readyz", http.StatusOK},
		{"/healthz", http.StatusOK},
		{"/api/v1/me", http.StatusServiceUnavailable},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "abc"})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != tc.want {
			t.Errorf("GET %s = %d, want %d", tc.path, w.Code, tc.want)
		}
	}
}
