package authprovider

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/url"
	"strconv"
	"strings"

	"github.com/fisk086/sya/internal/auth"
	"github.com/fisk086/sya/internal/config"
	"github.com/fisk086/sya/internal/larkauth"
	"github.com/fisk086/sya/internal/logger"
	"github.com/fisk086/sya/internal/model"
	"github.com/fisk086/sya/internal/schema"
	"github.com/fisk086/sya/internal/storage"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"gorm.io/gorm"
)

// LarkProvider implements Feishu / Lark OAuth (see aiops lark_routes.py).
type LarkProvider struct {
	userStore storage.UserStore
	jwtCfg    auth.JWTConfig
	settings  *config.Settings
}

// NewLarkProvider builds the Lark SSO backend.
func NewLarkProvider(userStore storage.UserStore, jwtCfg auth.JWTConfig, settings *config.Settings) *LarkProvider {
	return &LarkProvider{
		userStore: userStore,
		jwtCfg:    jwtCfg,
		settings:  settings,
	}
}

func (p *LarkProvider) ID() string { return "lark" }

func (p *LarkProvider) Ready() bool {
	return strings.TrimSpace(p.settings.SSOAppID) != "" &&
		strings.TrimSpace(p.settings.SSOAppSecret) != "" &&
		strings.TrimSpace(p.settings.SSORedirectURI) != ""
}

func (p *LarkProvider) larkClient() *larkauth.Client {
	return larkauth.NewClient(
		p.settings.SSOAppID,
		p.settings.SSOAppSecret,
		p.settings.SSORedirectURI,
		p.settings.SSOOpenAPIBase,
	)
}

func (p *LarkProvider) StartSSO(ctx context.Context, c *app.RequestContext) {
	_ = ctx
	if p.userStore == nil {
		c.JSON(consts.StatusServiceUnavailable, auth.H{"error": "user store unavailable (set DATABASE_URL)"})
		return
	}
	if !p.Ready() {
		c.JSON(consts.StatusServiceUnavailable, auth.H{"error": "Lark OAuth is not configured (SSO_APP_ID, SSO_APP_SECRET, SSO_REDIRECT_URI)"})
		return
	}
	state := larkauth.GenerateOAuthState(p.settings.JWTSecretKey)
	setOAuthStateCookie(c, oauthStateCookieName(p.ID()), state, p.settings.SSOCookieDomain)
	loginURL := p.larkClient().LoginURL(state)
	c.Redirect(consts.StatusFound, []byte(loginURL))
}

func (p *LarkProvider) SSOConfig(ctx context.Context, c *app.RequestContext) {
	_ = ctx
	if p.userStore == nil {
		c.JSON(consts.StatusServiceUnavailable, auth.H{"error": "user store unavailable"})
		return
	}
	if !p.Ready() {
		c.JSON(consts.StatusInternalServerError, auth.H{"error": "Lark auth configuration is incomplete"})
		return
	}
	state := larkauth.GenerateOAuthState(p.settings.JWTSecretKey)
	setOAuthStateCookie(c, oauthStateCookieName(p.ID()), state, p.settings.SSOCookieDomain)
	loginURL := p.larkClient().LoginURL(state)
	c.JSON(consts.StatusOK, auth.H{
		"provider":       p.ID(),
		"login_url":      loginURL,
		"lark_login_url": loginURL,
		"redirect_uri":   p.settings.SSORedirectURI,
	})
}

func (p *LarkProvider) HandleCallback(ctx context.Context, c *app.RequestContext, redirect bool) {
	_ = ctx
	if p.userStore == nil {
		c.JSON(consts.StatusServiceUnavailable, auth.H{"error": "user store unavailable"})
		return
	}
	if !p.Ready() {
		c.JSON(consts.StatusServiceUnavailable, auth.H{"error": "Lark OAuth is not configured"})
		return
	}
	code := strings.TrimSpace(string(c.QueryArgs().Peek("code")))
	state := strings.TrimSpace(string(c.QueryArgs().Peek("state")))
	cookieName := oauthStateCookieName(p.ID())
	cookieState := string(c.Request.Header.Cookie(cookieName))
	clearOAuthStateCookie(c, cookieName, p.settings.SSOCookieDomain)

	if err := larkauth.ValidateOAuthState(p.settings.JWTSecretKey, cookieState, state); err != nil {
		logger.Warn("lark oauth state invalid", "err", err)
		c.JSON(consts.StatusBadRequest, auth.H{"error": "invalid or expired OAuth state"})
		return
	}
	if code == "" {
		c.JSON(consts.StatusBadRequest, auth.H{"error": "missing code"})
		return
	}

	info, err := p.larkClient().UserInfoFromCode(code)
	if err != nil {
		logger.Error("lark user info failed", "err", err)
		c.JSON(consts.StatusBadRequest, auth.H{"error": "failed to get user info from Lark"})
		return
	}

	ok, err := p.allowUser(info.Email)
	if err != nil {
		logger.Error("lark whitelist check failed", "err", err)
		c.JSON(consts.StatusInternalServerError, auth.H{"error": "whitelist check failed"})
		return
	}
	if !ok {
		c.JSON(consts.StatusForbidden, auth.H{"error": "user not allowed to sign in (whitelist)"})
		return
	}

	user, err := p.upsertUser(info)
	if err != nil {
		logger.Error("lark upsert user failed", "err", err)
		c.JSON(consts.StatusInternalServerError, auth.H{"error": "failed to provision user"})
		return
	}
	if user.Status != model.UserStatusActive {
		c.JSON(consts.StatusForbidden, auth.H{"error": "user account is " + string(user.Status)})
		return
	}

	accessToken, err := auth.GenerateAccessToken(p.jwtCfg, user.ID, user.Username)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, auth.H{"error": "failed to generate access token"})
		return
	}
	refreshToken, err := auth.GenerateRefreshToken(p.jwtCfg, user.ID)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, auth.H{"error": "failed to generate refresh token"})
		return
	}

	if !redirect {
		c.JSON(consts.StatusOK, schema.TokenResponse{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			TokenType:    "bearer",
			ExpiresIn:    int64(p.jwtCfg.AccessExpire.Seconds()),
			User: schema.UserResponse{
				ID:        user.ID,
				Username:  user.Username,
				Email:     user.Email,
				FullName:  strValPtr(user.FullName),
				AvatarURL: strValPtr(user.AvatarURL),
				Status:    string(user.Status),
				IsAdmin:   user.IsAdmin,
			},
		})
		return
	}

	success := strings.TrimSpace(p.settings.SSOOAuthSuccessURL)
	if success == "" {
		c.JSON(consts.StatusOK, auth.H{
			"message":       "Lark login OK; set SSO_OAUTH_SUCCESS_REDIRECT for browser redirect",
			"access_token":  accessToken,
			"refresh_token": refreshToken,
			"token_type":    "bearer",
			"expires_in":    int64(p.jwtCfg.AccessExpire.Seconds()),
		})
		return
	}
	u, err := url.Parse(success)
	if err != nil || u.Scheme == "" || u.Host == "" {
		c.JSON(consts.StatusInternalServerError, auth.H{"error": "invalid SSO_OAUTH_SUCCESS_REDIRECT"})
		return
	}
	q := u.Query()
	q.Set("access_token", accessToken)
	q.Set("refresh_token", refreshToken)
	q.Set("token_type", "bearer")
	q.Set("expires_in", strconv.FormatInt(int64(p.jwtCfg.AccessExpire.Seconds()), 10))
	u.RawQuery = q.Encode()
	c.Redirect(consts.StatusFound, []byte(u.String()))
}

func (p *LarkProvider) allowUser(email string) (bool, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !p.settings.SSOAuthEnforceWhitelist {
		return true, nil
	}
	if p.isAdminEmail(email) {
		return true, nil
	}
	_, err := p.userStore.GetUserByEmail(email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (p *LarkProvider) isAdminEmail(email string) bool {
	raw := strings.TrimSpace(p.settings.SSOAdminEmails)
	if raw == "" {
		return false
	}
	for _, e := range strings.Split(raw, ",") {
		if strings.EqualFold(strings.TrimSpace(e), email) {
			return true
		}
	}
	return false
}

func (p *LarkProvider) upsertUser(info *larkauth.UserInfo) (*model.User, error) {
	email := info.Email
	user, err := p.userStore.GetUserByEmail(email)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if user != nil {
		user.FullName = ptrOrUpdate(user.FullName, info.Name)
		user.AvatarURL = ptrOrUpdate(user.AvatarURL, info.AvatarURL)
		if info.OpenID != "" {
			user.LarkOpenID = ptrCopy(info.OpenID)
		}
		if info.UnionID != "" {
			user.LarkUnionID = ptrCopy(info.UnionID)
		}
		if p.isAdminEmail(email) {
			user.IsAdmin = true
		}
		if err := p.userStore.UpdateUser(user); err != nil {
			return nil, err
		}
		return user, nil
	}
	if info.OpenID != "" {
		u2, err := p.userStore.GetUserByLarkOpenID(info.OpenID)
		if err == nil && u2 != nil {
			u2.Email = email
			u2.FullName = ptrOrUpdate(u2.FullName, info.Name)
			u2.AvatarURL = ptrOrUpdate(u2.AvatarURL, info.AvatarURL)
			u2.LarkUnionID = ptrOrUpdateStr(u2.LarkUnionID, info.UnionID)
			if p.isAdminEmail(email) {
				u2.IsAdmin = true
			}
			if err := p.userStore.UpdateUser(u2); err != nil {
				return nil, err
			}
			return u2, nil
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	username, err := p.allocUsername(email)
	if err != nil {
		return nil, err
	}
	randPw := randomURLSafe(32)
	newUser := &model.User{
		Username:    username,
		Email:       email,
		FullName:    ptrCopy(info.Name),
		AvatarURL:   ptrCopy(info.AvatarURL),
		LarkOpenID:  ptrCopy(info.OpenID),
		LarkUnionID: ptrCopy(info.UnionID),
		Status:      model.UserStatusActive,
		IsAdmin:     p.isAdminEmail(email),
	}
	if err := newUser.SetPassword(randPw); err != nil {
		return nil, err
	}
	if err := p.userStore.CreateUser(newUser); err != nil {
		return nil, err
	}
	return newUser, nil
}

func (p *LarkProvider) allocUsername(email string) (string, error) {
	local := strings.Split(email, "@")[0]
	base := strings.ReplaceAll(local, ".", "_")
	if base == "" {
		base = "user"
	}
	if len(base) > 50 {
		base = base[:50]
	}
	username := base
	for n := 0; n < 1000; n++ {
		_, err := p.userStore.GetUserByUsername(username)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return username, nil
		}
		if err != nil {
			return "", err
		}
		username = base + "_" + strconv.Itoa(n+1)
		if len(username) > 50 {
			username = username[:50]
		}
	}
	return "", errors.New("could not allocate username")
}

func ptrCopy(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

func ptrOrUpdate(cur *string, val string) *string {
	val = strings.TrimSpace(val)
	if val == "" {
		return cur
	}
	return &val
}

func ptrOrUpdateStr(cur *string, val string) *string {
	val = strings.TrimSpace(val)
	if val == "" {
		return cur
	}
	return &val
}

func randomURLSafe(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func strValPtr(s *string) string {
	if s == nil || strings.TrimSpace(*s) == "" {
		return ""
	}
	return strings.TrimSpace(*s)
}
