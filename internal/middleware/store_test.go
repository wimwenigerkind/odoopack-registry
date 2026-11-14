package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/apitoken"
	"github.com/wimwenigerkind/odoopack-registry/internal/auth"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
)

// downSessions behaves like a session store whose Redis is unreachable.
type downSessions struct{ gets int }

func (s *downSessions) Create(uuid.UUID, time.Duration, string, string) (auth.Session, error) {
	return auth.Session{}, fmt.Errorf("%w: connection refused", auth.ErrStoreUnavailable)
}

func (s *downSessions) Get(string) (auth.Session, error) {
	s.gets++
	return auth.Session{}, fmt.Errorf("%w: connection refused", auth.ErrStoreUnavailable)
}

func (s *downSessions) Delete(string) error {
	return fmt.Errorf("%w: connection refused", auth.ErrStoreUnavailable)
}

func (s *downSessions) Stop() {}

// whoami answers with the resolved user id, or "anonymous".
func whoami(c *gin.Context) {
	if id, ok := CurrentUserID(c); ok {
		c.String(http.StatusOK, id.String())
		return
	}
	c.String(http.StatusOK, "anonymous")
}

func serve(r *gin.Engine, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func withSessionCookie(req *http.Request) *http.Request {
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "abc"})
	return req
}

func TestStoreDownRegistryStaysAnonymous(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sessions := &downSessions{}
	r := gin.New()
	r.GET("/registry/v1/addons/*name", OptionalAuth(sessions), ApiKeyOptional(&fakeTokens{}), whoami)

	w := serve(r, withSessionCookie(httptest.NewRequest(http.MethodGet, "/registry/v1/addons/acme_base.json", nil)))
	if w.Code != http.StatusOK || w.Body.String() != "anonymous" {
		t.Errorf("got %d %q, want 200 anonymous", w.Code, w.Body.String())
	}
	if sessions.gets != 1 {
		t.Errorf("session store asked %d times, want 1", sessions.gets)
	}
}

func TestStoreDownBearerTokenStillWorks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userID := uuid.New()
	tokens := &fakeTokens{byHash: map[string]models.ApiToken{
		apitoken.Hash("odp_acme"): {Base: models.Base{ID: uuid.New()}, UserID: userID},
	}}
	r := gin.New()
	r.GET("/registry/v1/addons/*name", OptionalAuth(&downSessions{}), ApiKeyOptional(tokens), whoami)

	req := withSessionCookie(httptest.NewRequest(http.MethodGet, "/registry/v1/addons/acme_base.json", nil))
	req.Header.Set("Authorization", "Bearer odp_acme")
	w := serve(r, req)
	if w.Code != http.StatusOK || w.Body.String() != userID.String() {
		t.Errorf("got %d %q, want 200 %s", w.Code, w.Body.String(), userID)
	}
}

func TestStoreDownRequireAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/me", RequireAuth(&downSessions{}), whoami)

	w := serve(r, withSessionCookie(httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)))
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
	if body := strings.TrimSpace(w.Body.String()); body != `{"error":"session store unavailable"}` {
		t.Errorf("body = %s", body)
	}

	// Without a cookie the store is not asked, so the answer stays 401.
	w = serve(r, httptest.NewRequest(http.MethodGet, "/api/v1/me", nil))
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status without cookie = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestStoreDownRequireAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/users", RequireAdmin(&downSessions{}, nil), whoami)

	w := serve(r, withSessionCookie(httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)))
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}
