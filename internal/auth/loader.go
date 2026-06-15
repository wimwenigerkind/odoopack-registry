package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

func LoadProviders(ctx context.Context, baseURL string) (map[string]Provider, error) {
	names := collectProviderNames()
	out := make(map[string]Provider, len(names))

	for _, name := range names {
		providerType := viper.GetString("auth." + name + ".type")
		if providerType == "" {
			continue
		}
		callback := strings.TrimRight(baseURL, "/") + "/auth/" + name + "/callback"
		switch providerType {
		case string(OIDC):
			oc := OIDCConfig{
				Type:          providerType,
				AllowLogin:    viper.GetBool("auth." + name + ".allow_login"),
				AllowRegister: viper.GetBool("auth." + name + ".allow_register"),
				IssuerURL:     viper.GetString("auth." + name + ".issuer_url"),
				ClientID:      viper.GetString("auth." + name + ".client_id"),
				ClientSecret:  viper.GetString("auth." + name + ".client_secret"),
			}
			p, err := NewOIDCProvider(ctx, name, oc, callback)
			if err != nil {
				return nil, fmt.Errorf("auth.%s: %w", name, err)
			}
			out[name] = p
		case string(ProviderTypeGitHub):
			gc := GitHubConfig{
				AllowLogin:          viper.GetBool("auth." + name + ".allow_login"),
				AllowRegister:       viper.GetBool("auth." + name + ".allow_register"),
				AllowGitIntegration: viper.GetBool("auth." + name + ".allow_git_integration"),
				ClientID:            viper.GetString("auth." + name + ".client_id"),
				ClientSecret:        viper.GetString("auth." + name + ".client_secret"),
			}
			out[name] = NewGitHubProvider(name, gc, callback)
		case string(ProviderTypeBitbucket):
			bc := BitbucketConfig{
				AllowLogin:          viper.GetBool("auth." + name + ".allow_login"),
				AllowRegister:       viper.GetBool("auth." + name + ".allow_register"),
				AllowGitIntegration: viper.GetBool("auth." + name + ".allow_git_integration"),
				ClientID:            viper.GetString("auth." + name + ".client_id"),
				ClientSecret:        viper.GetString("auth." + name + ".client_secret"),
			}
			out[name] = NewBitbucketProvider(name, bc, callback)
		case string(ProviderTypeEntra):
			ec := EntraConfig{
				AllowLogin:    viper.GetBool("auth." + name + ".allow_login"),
				AllowRegister: viper.GetBool("auth." + name + ".allow_register"),
				TenantID:      viper.GetString("auth." + name + ".tenant_id"),
				ClientID:      viper.GetString("auth." + name + ".client_id"),
				ClientSecret:  viper.GetString("auth." + name + ".client_secret"),
			}
			p, err := NewEntraProvider(ctx, name, ec, callback)
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

func collectProviderNames() []string {
	seen := map[string]bool{}
	var names []string

	for name, cfgAny := range viper.GetStringMap("auth") {
		if _, ok := cfgAny.(map[string]any); !ok {
			continue
		}
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}

	for _, name := range strings.Split(viper.GetString("auth.providers"), ",") {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}

	return names
}
