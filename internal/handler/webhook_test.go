package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/apitoken"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
)

type fakeTokens struct{ token models.ApiToken }

func (f fakeTokens) GetByHash(hash string) (*models.ApiToken, error) {
	if hash != f.token.TokenHash {
		return nil, nil
	}
	return &f.token, nil
}

func (fakeTokens) TouchLastUsed(uuid.UUID, time.Time) error { return nil }

func TestWebhookRejectsAnOversizedBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tokens := fakeTokens{models.ApiToken{UserID: uuid.New(), TokenHash: apitoken.Hash(plainToken)}}
	h := NewWebhookHandler(nil, tokens, nil)

	body := `{"repository":{"full_name":"acme/addons"},"pad":"` + strings.Repeat("x", maxWebhookBytes) + `"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/webhooks/forgejo", strings.NewReader(body))
	c.Request.Header.Set("Authorization", "Bearer "+plainToken)
	c.Request.Header.Set("X-Forgejo-Event", "push")

	h.Forgejo(c)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413: %s", w.Code, w.Body)
	}
}
