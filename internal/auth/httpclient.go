package auth

import (
	"context"
	"net/http"
	"time"

	"golang.org/x/oauth2"
)

// providerTimeout bounds every call to an identity or git provider, so a
// provider that stops answering cannot hold a login or a build forever.
const providerTimeout = 15 * time.Second

var httpClient = &http.Client{Timeout: providerTimeout}

// withHTTPClient makes oauth2 and go-oidc use httpClient; both fall back to
// http.DefaultClient, which has no timeout, when the context carries none.
func withHTTPClient(ctx context.Context) context.Context {
	return context.WithValue(ctx, oauth2.HTTPClient, httpClient)
}
