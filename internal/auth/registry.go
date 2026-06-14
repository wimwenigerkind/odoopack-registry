package auth

import "fmt"

type Registry struct {
	all          map[string]Provider
	logins       map[string]LoginProvider
	integrations map[string]IntegrationProvider
}

func NewRegistry(providers map[string]Provider) *Registry {
	r := &Registry{
		all:          providers,
		logins:       make(map[string]LoginProvider),
		integrations: make(map[string]IntegrationProvider),
	}
	for name, p := range providers {
		if lp, ok := p.(LoginProvider); ok && p.AllowLogin() {
			r.logins[name] = lp
		}
		if ip, ok := p.(IntegrationProvider); ok && p.AllowGitIntegration() {
			r.integrations[name] = ip
		}
	}
	return r
}

func (r *Registry) Get(name string) (Provider, error) {
	p, ok := r.all[name]
	if !ok {
		return nil, fmt.Errorf("auth: provider %q not configured", name)
	}
	return p, nil
}

func (r *Registry) GetLogin(name string) (LoginProvider, error) {
	p, ok := r.logins[name]
	if !ok {
		return nil, fmt.Errorf("auth: login provider %q not configured", name)
	}
	return p, nil
}

func (r *Registry) GetIntegration(name string) (IntegrationProvider, error) {
	p, ok := r.integrations[name]
	if !ok {
		return nil, fmt.Errorf("auth: integration provider %q not configured", name)
	}
	return p, nil
}

func (r *Registry) EnabledForLogin() []LoginProvider {
	out := make([]LoginProvider, 0, len(r.logins))
	for _, p := range r.logins {
		out = append(out, p)
	}
	return out
}

func (r *Registry) EnabledForIntegration() []IntegrationProvider {
	out := make([]IntegrationProvider, 0, len(r.integrations))
	for _, p := range r.integrations {
		out = append(out, p)
	}
	return out
}
