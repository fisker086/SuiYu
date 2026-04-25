package authprovider

import (
	"context"

	"github.com/fisk086/sya/internal/auth"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// StubProvider is a placeholder for SSO channels not yet implemented (dingtalk, wecom, telegram, ...).
type StubProvider struct {
	id string
}

// NewStubProvider returns a provider that answers with HTTP 501 Not Implemented.
func NewStubProvider(id string) *StubProvider {
	return &StubProvider{id: id}
}

func (s *StubProvider) ID() string { return s.id }

func (s *StubProvider) Ready() bool { return false }

func (s *StubProvider) StartSSO(ctx context.Context, c *app.RequestContext) {
	_ = ctx
	c.JSON(consts.StatusNotImplemented, auth.H{
		"error":   "sso not implemented for provider: " + s.id,
		"provider": s.id,
	})
}

func (s *StubProvider) SSOConfig(ctx context.Context, c *app.RequestContext) {
	_ = ctx
	c.JSON(consts.StatusNotImplemented, auth.H{
		"error":    "sso not implemented for provider: " + s.id,
		"provider": s.id,
	})
}

func (s *StubProvider) HandleCallback(ctx context.Context, c *app.RequestContext, redirect bool) {
	_ = ctx
	_ = redirect
	c.JSON(consts.StatusNotImplemented, auth.H{
		"error":    "sso not implemented for provider: " + s.id,
		"provider": s.id,
	})
}
