package auth

import "fmt"

type Registry struct {
	providers map[string]Provider
}

func NewRegistry(providers map[string]Provider) *Registry {
	return &Registry{providers: providers}
}

func (r *Registry) Get(name string) (Provider, error) {
	p, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("auth: provider %q not configured", name)
	}
	return p, nil
}

func (r *Registry) EnabledForLogin() []Provider {
	out := make([]Provider, 0, len(r.providers))
	for _, p := range r.providers {
		if p.AllowLogin() {
			out = append(out, p)
		}
	}
	return out
}
