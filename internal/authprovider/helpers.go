package authprovider

import (
	"net/http"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
)

func bytesIsHTTPS(c *app.RequestContext) bool {
	if string(c.Request.Header.Peek("X-Forwarded-Proto")) == "https" {
		return true
	}
	return string(c.Request.URI().Scheme()) == "https"
}

func setOAuthStateCookie(c *app.RequestContext, name, value, domain string) {
	secure := bytesIsHTTPS(c)
	ck := &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   600,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
	if d := strings.TrimSpace(domain); d != "" {
		ck.Domain = d
	}
	c.Response.Header.Set("Set-Cookie", ck.String())
}

func clearOAuthStateCookie(c *app.RequestContext, name, domain string) {
	secure := bytesIsHTTPS(c)
	ck := &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
	if d := strings.TrimSpace(domain); d != "" {
		ck.Domain = d
	}
	c.Response.Header.Set("Set-Cookie", ck.String())
}

func oauthStateCookieName(providerID string) string {
	return "sya_oauth_state_" + providerID
}
