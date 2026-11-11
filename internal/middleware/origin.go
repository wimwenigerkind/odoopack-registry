package middleware

import (
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// SameOrigin refuses state-changing requests that a browser sent from a page
// outside origins, so a foreign site cannot ride on the session cookie.
// Requests with a bearer token pass: a browser adds no Authorization header to
// a cross-site request on its own, and the CLI and webhooks send no Origin.
func SameOrigin(origins []string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(origins))
	for _, o := range origins {
		if n := normalizeOrigin(o); n != "" {
			allowed[n] = true
		}
	}
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}
		if strings.HasPrefix(c.GetHeader("Authorization"), "Bearer ") {
			c.Next()
			return
		}
		// Browsers send Origin on every non-GET fetch; Sec-Fetch-Site covers
		// the rare request where it is missing.
		if origin := c.GetHeader("Origin"); origin != "" {
			if allowed[normalizeOrigin(origin)] {
				c.Next()
				return
			}
		} else if c.GetHeader("Sec-Fetch-Site") == "same-origin" {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "cross-origin request refused"})
	}
}

// normalizeOrigin reduces a URL to scheme://host[:port] the way a browser
// sends it in Origin: lower case, no path and no default port.
func normalizeOrigin(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	defaultPort := (scheme == "https" && port == "443") || (scheme == "http" && port == "80")
	switch {
	case port != "" && !defaultPort:
		host = net.JoinHostPort(host, port)
	case strings.Contains(host, ":"):
		host = "[" + host + "]"
	}
	return scheme + "://" + host
}
