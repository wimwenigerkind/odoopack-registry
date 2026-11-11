package handler

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin/binding"
	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/apitoken"
)

const plainToken = "odp_0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestNewApiToken(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	userID := uuid.New()

	tok := newApiToken(userID, createTokenRequest{Name: "ci", ExpiresInDays: 30}, plainToken, now)
	if tok.UserID != userID || tok.Name != "ci" {
		t.Errorf("owner or name not taken over: %+v", tok)
	}
	if tok.TokenHash != apitoken.Hash(plainToken) {
		t.Errorf("TokenHash = %s, want the sha256 of the token", tok.TokenHash)
	}
	if tok.Prefix != "odp_0123" || tok.LastFour != "cdef" {
		t.Errorf("display parts = %q, %q", tok.Prefix, tok.LastFour)
	}
	if want := now.AddDate(0, 0, 30); tok.ExpiresAt == nil || !tok.ExpiresAt.Equal(want) {
		t.Errorf("ExpiresAt = %v, want %v", tok.ExpiresAt, want)
	}

	if tok := newApiToken(userID, createTokenRequest{Name: "ci"}, plainToken, now); tok.ExpiresAt != nil {
		t.Errorf("token without expires_in_days expires at %v", tok.ExpiresAt)
	}
}

func TestCreateTokenRequestValidation(t *testing.T) {
	tests := []struct {
		body string
		ok   bool
	}{
		{`{"name":"ci"}`, true},
		{`{"name":"ci","expires_in_days":90}`, true},
		{`{"name":"ci","expires_in_days":0}`, true},
		{`{"name":"ci","expires_in_days":-1}`, false},
		{`{"name":"ci","expires_in_days":3651}`, false},
		{`{"expires_in_days":30}`, false},
	}
	for _, tt := range tests {
		var req createTokenRequest
		if err := json.Unmarshal([]byte(tt.body), &req); err != nil {
			t.Fatal(err)
		}
		err := binding.Validator.ValidateStruct(&req)
		if (err == nil) != tt.ok {
			t.Errorf("%s: err = %v, want ok = %v", tt.body, err, tt.ok)
		}
	}
}

func TestTokenResponses(t *testing.T) {
	tok := newApiToken(uuid.New(), createTokenRequest{Name: "ci"}, plainToken, time.Now())

	created, err := json.Marshal(createTokenResponse{ApiToken: tok, Token: plainToken})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(created), `"token":"`+plainToken+`"`) {
		t.Errorf("create response lacks the token: %s", created)
	}

	listed, err := json.Marshal(tok)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{plainToken, tok.TokenHash, `"token`} {
		if strings.Contains(string(listed), secret) {
			t.Errorf("list entry contains %q: %s", secret, listed)
		}
	}
	for _, field := range []string{`"prefix":"odp_0123"`, `"last_four":"cdef"`} {
		if !strings.Contains(string(listed), field) {
			t.Errorf("list entry lacks %s: %s", field, listed)
		}
	}
}
