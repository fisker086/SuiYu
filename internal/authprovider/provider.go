// Package authprovider defines pluggable SSO backends (Lark, DingTalk, WeCom, ...).
package authprovider

import (
	"context"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
)

// SSOProvider is a single sign-on channel. Password login stays on POST /auth/login.
type SSOProvider interface {
	// ID is the stable name used in URLs and AUTH_TYPE (e.g. "lark", "dingtalk", "wecom", "telegram").
	ID() string
	// Ready is true when configuration is sufficient to run OAuth for this provider.
	Ready() bool
	// StartSSO sets CSRF state (cookie) and redirects the browser to the IdP (or JSON error).
	StartSSO(ctx context.Context, c *app.RequestContext)
	// SSOConfig returns JSON for clients that need the authorize URL before redirect (optional).
	SSOConfig(ctx context.Context, c *app.RequestContext)
	// HandleCallback exchanges code for local JWT; redirect=true appends tokens to success URL or returns JSON.
	HandleCallback(ctx context.Context, c *app.RequestContext, redirect bool)
}

// Registry holds registered SSO providers by ID.
type Registry struct {
	byID map[string]SSOProvider
	ids  []string
}

// NewRegistry builds a registry with the given providers (order preserved).
func NewRegistry(providers ...SSOProvider) *Registry {
	r := &Registry{byID: make(map[string]SSOProvider)}
	for _, p := range providers {
		if p == nil {
			continue
		}
		id := strings.ToLower(strings.TrimSpace(p.ID()))
		if id == "" {
			continue
		}
		if _, dup := r.byID[id]; dup {
			continue
		}
		r.byID[id] = p
		r.ids = append(r.ids, id)
	}
	return r
}

// Get returns the provider and true if registered.
func (r *Registry) Get(id string) (SSOProvider, bool) {
	id = strings.ToLower(strings.TrimSpace(id))
	p, ok := r.byID[id]
	return p, ok
}

// IDs returns registered provider ids in registration order.
func (r *Registry) IDs() []string {
	out := make([]string, len(r.ids))
	copy(out, r.ids)
	return out
}
