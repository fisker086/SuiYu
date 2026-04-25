package controller

import (
	"context"
	"strings"

	"github.com/fisk086/sya/internal/auth"
	"github.com/fisk086/sya/internal/authprovider"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// SSOController dispatches multi-provider browser SSO (Lark, DingTalk, WeCom, …) via authprovider.Registry.
type SSOController struct {
	reg *authprovider.Registry
}

// NewSSOController wires SSO routes; reg may be nil (no routes registered).
func NewSSOController(reg *authprovider.Registry) *SSOController {
	return &SSOController{reg: reg}
}

func (s *SSOController) RegisterRoutes(r *server.Hertz) {
	if s == nil || s.reg == nil {
		return
	}
	g := r.Group("/api/v1/auth")
	g.GET("/sso/:provider", s.start)
	g.GET("/sso/:provider/callback", s.callback)
	g.GET("/sso/:provider/callback-token", s.callbackToken)
	g.GET("/sso/:provider/config", s.config)
}

func (s *SSOController) providerFrom(c *app.RequestContext) (authprovider.SSOProvider, bool) {
	id := strings.ToLower(strings.TrimSpace(c.Param("provider")))
	return s.reg.Get(id)
}

func (s *SSOController) start(ctx context.Context, c *app.RequestContext) {
	p, ok := s.providerFrom(c)
	if !ok {
		c.JSON(consts.StatusNotFound, auth.H{"error": "unknown SSO provider"})
		return
	}
	p.StartSSO(ctx, c)
}

func (s *SSOController) callback(ctx context.Context, c *app.RequestContext) {
	p, ok := s.providerFrom(c)
	if !ok {
		c.JSON(consts.StatusNotFound, auth.H{"error": "unknown SSO provider"})
		return
	}
	p.HandleCallback(ctx, c, true)
}

func (s *SSOController) callbackToken(ctx context.Context, c *app.RequestContext) {
	p, ok := s.providerFrom(c)
	if !ok {
		c.JSON(consts.StatusNotFound, auth.H{"error": "unknown SSO provider"})
		return
	}
	p.HandleCallback(ctx, c, false)
}

func (s *SSOController) config(ctx context.Context, c *app.RequestContext) {
	p, ok := s.providerFrom(c)
	if !ok {
		c.JSON(consts.StatusNotFound, auth.H{"error": "unknown SSO provider"})
		return
	}
	p.SSOConfig(ctx, c)
}
