package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSameOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mw := SameOrigin([]string{"https://packages.example.com/", "http://localhost:5173"})

	for _, tc := range []struct {
		name    string
		method  string
		headers map[string]string
		want    int
	}{
		{"same origin", http.MethodPost, map[string]string{"Origin": "https://packages.example.com"}, http.StatusOK},
		{"same origin with default port", http.MethodDelete, map[string]string{"Origin": "https://packages.example.com:443"}, http.StatusOK},
		{"allowed cors origin", http.MethodPut, map[string]string{"Origin": "http://localhost:5173"}, http.StatusOK},
		{"foreign origin", http.MethodPost, map[string]string{"Origin": "https://evil.example.com"}, http.StatusForbidden},
		{"foreign port", http.MethodPatch, map[string]string{"Origin": "http://localhost:6969"}, http.StatusForbidden},
		{"opaque origin", http.MethodPost, map[string]string{"Origin": "null"}, http.StatusForbidden},
		{"foreign origin wins over same-origin fetch site", http.MethodPost, map[string]string{"Origin": "https://evil.example.com", "Sec-Fetch-Site": "same-origin"}, http.StatusForbidden},
		{"no origin, fetch site same-origin", http.MethodPost, map[string]string{"Sec-Fetch-Site": "same-origin"}, http.StatusOK},
		{"no origin, fetch site same-site", http.MethodPost, map[string]string{"Sec-Fetch-Site": "same-site"}, http.StatusForbidden},
		{"no origin, fetch site cross-site", http.MethodPost, map[string]string{"Sec-Fetch-Site": "cross-site"}, http.StatusForbidden},
		{"no origin, no fetch site", http.MethodDelete, nil, http.StatusForbidden},
		{"bearer token without origin", http.MethodPost, map[string]string{"Authorization": "Bearer odp_example"}, http.StatusOK},
		{"bearer token from foreign origin", http.MethodPost, map[string]string{"Authorization": "Bearer odp_example", "Origin": "https://evil.example.com"}, http.StatusOK},
		{"basic auth is no bearer token", http.MethodPost, map[string]string{"Authorization": "Basic dXNlcjpwYXNz"}, http.StatusForbidden},
		{"get from foreign origin", http.MethodGet, map[string]string{"Origin": "https://evil.example.com"}, http.StatusOK},
		{"get without headers", http.MethodGet, nil, http.StatusOK},
		{"head without headers", http.MethodHead, nil, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			r.Use(mw)
			r.Handle(tc.method, "/x", func(c *gin.Context) { c.Status(http.StatusOK) })

			req := httptest.NewRequest(tc.method, "/x", nil)
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Errorf("status = %d, want %d", w.Code, tc.want)
			}
		})
	}
}

func TestNormalizeOrigin(t *testing.T) {
	for in, want := range map[string]string{
		"https://Packages.Example.com/":    "https://packages.example.com",
		"https://packages.example.com:443": "https://packages.example.com",
		"http://localhost:6969/registry":   "http://localhost:6969",
		"http://[::1]:80":                  "http://[::1]",
		"http://[::1]:8080":                "http://[::1]:8080",
		"null":                             "",
		"":                                 "",
	} {
		if got := normalizeOrigin(in); got != want {
			t.Errorf("normalizeOrigin(%q) = %q, want %q", in, got, want)
		}
	}
}
