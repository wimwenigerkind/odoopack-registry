package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestLoggerDropsQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var buf bytes.Buffer
	r := gin.New()
	r.Use(RequestLogger(slog.New(slog.NewTextHandler(&buf, nil))))
	r.GET("/auth/:provider/callback", func(c *gin.Context) { c.Status(http.StatusFound) })

	req := httptest.NewRequest(http.MethodGet, "/auth/github/callback?code=secret-code&state=secret-state", nil)
	req.RemoteAddr = "192.0.2.10:51234"
	r.ServeHTTP(httptest.NewRecorder(), req)

	line := buf.String()
	for _, want := range []string{"method=GET", "path=/auth/github/callback ", "status=302", "latency=", "client_ip=192.0.2.10"} {
		if !strings.Contains(line, want) {
			t.Errorf("log line %q misses %q", line, want)
		}
	}
	for _, leak := range []string{"secret-code", "secret-state", "?"} {
		if strings.Contains(line, leak) {
			t.Errorf("log line %q leaks %q", line, leak)
		}
	}
}
