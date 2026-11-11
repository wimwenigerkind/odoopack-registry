package middleware

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/apitoken"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"github.com/wimwenigerkind/odoopack-registry/internal/repository"
)

type fakeTokens struct {
	byHash  map[string]models.ApiToken
	touched []uuid.UUID
}

func (f *fakeTokens) GetByHash(hash string) (*models.ApiToken, error) {
	tok, ok := f.byHash[hash]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &tok, nil
}

func (f *fakeTokens) TouchLastUsed(id uuid.UUID, _ time.Time) error {
	f.touched = append(f.touched, id)
	return nil
}

func TestBearerUserID(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)
	userID := uuid.New()
	const (
		valid   = "odp_valid"
		expired = "odp_expired"
		later   = "odp_later"
	)
	store := &fakeTokens{byHash: map[string]models.ApiToken{
		apitoken.Hash(valid):   {Base: models.Base{ID: uuid.New()}, UserID: userID},
		apitoken.Hash(expired): {Base: models.Base{ID: uuid.New()}, UserID: userID, ExpiresAt: &past},
		apitoken.Hash(later):   {Base: models.Base{ID: uuid.New()}, UserID: userID, ExpiresAt: &future},
	}}

	tests := []struct {
		name   string
		header string
		ok     bool
	}{
		{"valid token", "Bearer " + valid, true},
		{"token with an expiry ahead", "Bearer " + later, true},
		{"expired token", "Bearer " + expired, false},
		{"unknown token", "Bearer odp_unknown", false},
		{"stored hash sent as token", "Bearer " + apitoken.Hash(valid), false},
		{"no bearer scheme", valid, false},
		{"empty token", "Bearer  ", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store.touched = nil
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("GET", "/", nil)
			c.Request.Header.Set("Authorization", tt.header)

			got, ok := BearerUserID(c, store, now)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if ok && got != userID {
				t.Errorf("user = %s, want %s", got, userID)
			}
			wantTouched := 0
			if tt.ok {
				wantTouched = 1
			}
			if len(store.touched) != wantTouched {
				t.Errorf("last used touched %d times, want %d", len(store.touched), wantTouched)
			}
		})
	}
}
