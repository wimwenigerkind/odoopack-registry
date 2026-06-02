package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
)

func LoadProviders(ctx context.Context, baseURL string) (map[string]Provider, error) {
	raw := viper.GetStringMap("auth")
	out := make(map[string]Provider, len(raw))

	for name, cfgAny := range raw {
		cfg, ok := cfgAny.(map[string]any)
		if !ok {
			// Non-map entries under `auth.*` are config options
			// (e.g., cookie_secure), not provider definitions.
			continue
		}
		providerType, _ := cfg["type"].(string)
		if providerType == "" {
			continue
		}
		switch providerType {
		case string(OIDC):
			var oc OIDCConfig
			if err := mapstructure.Decode(cfg, &oc); err != nil {
				return nil, fmt.Errorf("auth.%s: decode: %w", name, err)
			}
			redirect := strings.TrimRight(baseURL, "/") + "/auth/" + name + "/callback"
			p, err := NewOIDCProvider(ctx, name, oc, redirect)
			if err != nil {
				return nil, fmt.Errorf("auth.%s: %w", name, err)
			}
			out[name] = p
		default:
			return nil, fmt.Errorf("auth.%s: unknown type %q", name, providerType)
		}
	}
	return out, nil
}
