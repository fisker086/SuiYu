package controller

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/fisk086/sya/internal/auth"
	"github.com/fisk086/sya/internal/captcha"
	"github.com/fisk086/sya/internal/logger"
	"github.com/fisk086/sya/internal/model"
	"github.com/fisk086/sya/internal/schema"
	"github.com/fisk086/sya/internal/storage"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"gorm.io/gorm"
)

type AuthController struct {
	userStore      storage.UserStore
	jwtCfg         auth.JWTConfig
	disableCaptcha bool
	authType       string // password | lark | dingtalk | wecom | telegram
}

func NewAuthController(userStore storage.UserStore, jwtCfg auth.JWTConfig, disableCaptcha bool, authType string) *AuthController {
	return &AuthController{
		userStore:      userStore,
		jwtCfg:         jwtCfg,
		disableCaptcha: disableCaptcha,
		authType:       authType,
	}
}

func (ctrl *AuthController) RegisterRoutes(r *server.Hertz) {
	authGroup := r.Group("/api/v1/auth")

	authGroup.GET("/captcha", ctrl.Captcha)
	authGroup.GET("/config", ctrl.GetAuthConfig) // 新增：获取登录配置
	authGroup.POST("/register", ctrl.Register)
	authGroup.POST("/login", ctrl.Login)
	authGroup.POST("/refresh", ctrl.RefreshToken)

	protected := authGroup.Group("")
	protected.Use(auth.JWTMiddleware(ctrl.jwtCfg, ctrl.getUserForMiddleware))
	protected.GET("/me", ctrl.GetMe)
	protected.PUT("/me", ctrl.UpdateMe)
	protected.POST("/change-password", ctrl.ChangePassword)

	admin := authGroup.Group("")
	admin.Use(auth.JWTMiddleware(ctrl.jwtCfg, ctrl.getUserForMiddleware))
	admin.GET("/users", ctrl.ListUsers)
}

func (ctrl *AuthController) getUserForMiddleware(userID int64) (*auth.User, error) {
	user, err := ctrl.userStore.GetUserByID(userID)
	if err != nil {
		return nil, err
	}
	return &auth.User{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
		Status:   string(user.Status),
		IsAdmin:  user.IsAdmin,
	}, nil
}

// @Summary Register a new user
// @Description Create an account with username, email, and password
// @Tags auth
// @Accept json
// @Produce json
// @Param request body schema.RegisterRequest true "Registration payload"
// @Success 201 {object} schema.RegisterSuccessResponse
// @Failure 400 {object} schema.AuthErrorResponse
// @Failure 409 {object} schema.AuthErrorResponse
// @Failure 500 {object} schema.AuthErrorResponse
// @Router /auth/register [post]
// GetAuthConfig returns auth configuration for the frontend (auth_type, captcha settings)
func (ctrl *AuthController) GetAuthConfig(ctx context.Context, c *app.RequestContext) {
	captchaDisabled := ctrl.disableCaptcha || ctrl.authType != "password"
	c.JSON(200, auth.H{
		"auth_type":        ctrl.authType,
		"captcha_disabled": captchaDisabled,
	})
}

// Captcha returns a numeric captcha image (SVG as data URL) and a one-time token for POST /auth/login.
func (ctrl *AuthController) Captcha(ctx context.Context, c *app.RequestContext) {
	token, image := captcha.Create()
	logger.Debug("captcha issued",
		"client_ip", c.ClientIP(),
		"user_agent", string(c.GetHeader("User-Agent")),
		"origin", string(c.GetHeader("Origin")))
	c.JSON(200, auth.H{
		"token": token,
		"image": image,
	})
}

func (ctrl *AuthController) Register(ctx context.Context, c *app.RequestContext) {
	var req schema.RegisterRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, auth.H{"error": "invalid request body"})
		return
	}

	if !ctrl.disableCaptcha && ctrl.authType == "password" {
		if strings.TrimSpace(req.CaptchaToken) == "" || strings.TrimSpace(req.CaptchaCode) == "" {
			c.JSON(400, auth.H{"error": "请输入验证码"})
			return
		}
		ok, captchaErr := captcha.Verify(req.CaptchaToken, req.CaptchaCode)
		if !ok {
			c.JSON(400, auth.H{"error": captchaErr})
			return
		}
	}

	if len(req.Password) < 6 {
		c.JSON(400, auth.H{"error": "password must be at least 6 characters"})
		return
	}

	_, err := ctrl.userStore.GetUserByUsername(req.Username)
	if err == nil {
		c.JSON(409, auth.H{"error": "username already exists"})
		return
	}

	_, err = ctrl.userStore.GetUserByEmail(req.Email)
	if err == nil {
		c.JSON(409, auth.H{"error": "email already exists"})
		return
	}

	user := &model.User{
		Username: req.Username,
		Email:    req.Email,
		Status:   model.UserStatusActive,
	}
	if req.FullName != "" {
		user.FullName = &req.FullName
	}
	if err := user.SetPassword(req.Password); err != nil {
		c.JSON(500, auth.H{"error": "failed to process password"})
		return
	}

	if err := ctrl.userStore.CreateUser(user); err != nil {
		logger.Error("failed to register user", "err", err)
		c.JSON(500, auth.H{"error": "registration failed"})
		return
	}

	logger.Info("user registered", "username", user.Username)
	c.JSON(201, auth.H{
		"message": "user registered successfully",
		"user": schema.UserResponse{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
			FullName: strVal(user.FullName),
			Status:   string(user.Status),
			IsAdmin:  user.IsAdmin,
		},
	})
}

// @Summary Login
// @Description Authenticate and receive access and refresh tokens
// @Tags auth
// @Accept json
// @Produce json
// @Param request body schema.LoginRequest true "Credentials"
// @Success 200 {object} schema.TokenResponse
// @Failure 400 {object} schema.AuthErrorResponse
// @Failure 401 {object} schema.AuthErrorResponse
// @Failure 403 {object} schema.AuthErrorResponse
// @Failure 500 {object} schema.AuthErrorResponse
// @Router /auth/login [post]
func (ctrl *AuthController) Login(ctx context.Context, c *app.RequestContext) {
	ip := c.ClientIP()
	ua := string(c.GetHeader("User-Agent"))
	origin := string(c.GetHeader("Origin"))
	referer := string(c.GetHeader("Referer"))
	contentType := string(c.ContentType())

	var req schema.LoginRequest
	if err := c.BindJSON(&req); err != nil {
		logger.Warn("login rejected: invalid JSON body",
			"client_ip", ip, "user_agent", ua, "origin", origin, "referer", referer,
			"content_type", contentType, "err", err)
		c.JSON(400, auth.H{"error": "invalid request body"})
		return
	}

	username := strings.TrimSpace(req.Username)
	logger.Info("login attempt",
		"client_ip", ip, "user_agent", ua, "origin", origin,
		"username", username,
		"captcha_disabled", ctrl.disableCaptcha,
		"has_captcha_token", strings.TrimSpace(req.CaptchaToken) != "",
		"has_captcha_code", strings.TrimSpace(req.CaptchaCode) != "",
		"has_password", len(req.Password) > 0)

	if !ctrl.disableCaptcha && ctrl.authType == "password" {
		if strings.TrimSpace(req.CaptchaToken) == "" || strings.TrimSpace(req.CaptchaCode) == "" {
			logger.Warn("login rejected: captcha missing",
				"client_ip", ip, "user_agent", ua, "username", username,
				"has_token", strings.TrimSpace(req.CaptchaToken) != "",
				"has_code", strings.TrimSpace(req.CaptchaCode) != "")
			c.JSON(400, auth.H{"error": "请输入验证码"})
			return
		}
		ok, captchaErr := captcha.Verify(req.CaptchaToken, req.CaptchaCode)
		if !ok {
			logger.Warn("login rejected: captcha verify failed",
				"client_ip", ip, "user_agent", ua, "username", username, "captcha_err", captchaErr)
			c.JSON(400, auth.H{"error": captchaErr})
			return
		}
	}

	user, err := ctrl.userStore.GetUserByUsername(req.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Warn("login failed: user not found",
				"client_ip", ip, "user_agent", ua, "username", username)
		} else {
			logger.Error("login failed: user lookup error",
				"client_ip", ip, "username", username, "err", err)
		}
		c.JSON(401, auth.H{"error": "invalid username or password"})
		return
	}
	if !user.VerifyPassword(req.Password) {
		logger.Warn("login failed: invalid password",
			"client_ip", ip, "user_agent", ua, "username", username)
		c.JSON(401, auth.H{"error": "invalid username or password"})
		return
	}

	if user.Status != model.UserStatusActive {
		logger.Warn("login rejected: user not active",
			"client_ip", ip, "user_agent", ua, "username", username, "status", string(user.Status))
		c.JSON(403, auth.H{"error": "user account is " + string(user.Status)})
		return
	}

	now := time.Now()
	_ = ctrl.userStore.UpdateLastLogin(user.ID, now)

	accessToken, err := auth.GenerateAccessToken(ctrl.jwtCfg, user.ID, user.Username)
	if err != nil {
		logger.Error("login failed: generate access token", "user_id", user.ID, "err", err)
		c.JSON(500, auth.H{"error": "failed to generate access token"})
		return
	}

	refreshToken, err := auth.GenerateRefreshToken(ctrl.jwtCfg, user.ID)
	if err != nil {
		logger.Error("login failed: generate refresh token", "user_id", user.ID, "err", err)
		c.JSON(500, auth.H{"error": "failed to generate refresh token"})
		return
	}

	logger.Info("login ok",
		"user_id", user.ID, "username", user.Username,
		"client_ip", ip, "user_agent", ua, "origin", origin)
	c.JSON(200, schema.TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "bearer",
		ExpiresIn:    int64(ctrl.jwtCfg.AccessExpire.Seconds()),
		User: schema.UserResponse{
			ID:        user.ID,
			Username:  user.Username,
			Email:     user.Email,
			FullName:  strVal(user.FullName),
			AvatarURL: strVal(user.AvatarURL),
			Status:    string(user.Status),
			IsAdmin:   user.IsAdmin,
		},
	})
}

// @Summary Refresh tokens
// @Description Exchange a refresh token for new access and refresh tokens
// @Tags auth
// @Accept json
// @Produce json
// @Param request body schema.RefreshTokenRequest true "Refresh token"
// @Success 200 {object} schema.TokenPairResponse
// @Failure 400 {object} schema.AuthErrorResponse
// @Failure 401 {object} schema.AuthErrorResponse
// @Failure 500 {object} schema.AuthErrorResponse
// @Router /auth/refresh [post]
func (ctrl *AuthController) RefreshToken(ctx context.Context, c *app.RequestContext) {
	var req schema.RefreshTokenRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, auth.H{"error": "invalid request body"})
		return
	}

	claims, err := auth.ParseToken(ctrl.jwtCfg, req.RefreshToken)
	if err != nil {
		c.JSON(401, auth.H{"error": "invalid or expired refresh token"})
		return
	}

	if claims.Type != "refresh" {
		c.JSON(401, auth.H{"error": "invalid token type"})
		return
	}

	user, err := ctrl.userStore.GetUserByID(claims.UserID)
	if err != nil || user.Status != model.UserStatusActive {
		c.JSON(401, auth.H{"error": "user not found or inactive"})
		return
	}

	accessToken, err := auth.GenerateAccessToken(ctrl.jwtCfg, user.ID, user.Username)
	if err != nil {
		c.JSON(500, auth.H{"error": "failed to generate access token"})
		return
	}

	newRefreshToken, err := auth.GenerateRefreshToken(ctrl.jwtCfg, user.ID)
	if err != nil {
		c.JSON(500, auth.H{"error": "failed to generate refresh token"})
		return
	}

	c.JSON(200, auth.H{
		"access_token":  accessToken,
		"refresh_token": newRefreshToken,
		"token_type":    "bearer",
		"expires_in":    int64(ctrl.jwtCfg.AccessExpire.Seconds()),
	})
}

// @Summary Current user profile
// @Description Returns the authenticated user (requires Bearer token)
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} schema.GetMeResponse
// @Failure 401 {object} schema.AuthErrorResponse
// @Failure 404 {object} schema.AuthErrorResponse
// @Router /auth/me [get]
func (ctrl *AuthController) GetMe(ctx context.Context, c *app.RequestContext) {
	user := auth.GetCurrentUser(c)
	if user == nil {
		c.JSON(401, auth.H{"error": "not authenticated"})
		return
	}

	fullUser, err := ctrl.userStore.GetUserByID(user.ID)
	if err != nil {
		c.JSON(404, auth.H{"error": "user not found"})
		return
	}

	c.JSON(200, auth.H{
		"user": schema.UserResponse{
			ID:        fullUser.ID,
			Username:  fullUser.Username,
			Email:     fullUser.Email,
			FullName:  strVal(fullUser.FullName),
			AvatarURL: strVal(fullUser.AvatarURL),
			Status:    string(fullUser.Status),
			IsAdmin:   fullUser.IsAdmin,
		},
	})
}

// @Summary Update current user profile
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body schema.UpdateProfileRequest true "Profile fields"
// @Success 200 {object} schema.ProfileUpdateResponse
// @Failure 400 {object} schema.AuthErrorResponse
// @Failure 401 {object} schema.AuthErrorResponse
// @Failure 404 {object} schema.AuthErrorResponse
// @Failure 409 {object} schema.AuthErrorResponse
// @Failure 500 {object} schema.AuthErrorResponse
// @Router /auth/me [put]
func (ctrl *AuthController) UpdateMe(ctx context.Context, c *app.RequestContext) {
	user := auth.GetCurrentUser(c)
	if user == nil {
		c.JSON(401, auth.H{"error": "not authenticated"})
		return
	}

	var req schema.UpdateProfileRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, auth.H{"error": "invalid request body"})
		return
	}

	fullUser, err := ctrl.userStore.GetUserByID(user.ID)
	if err != nil {
		c.JSON(404, auth.H{"error": "user not found"})
		return
	}

	if req.Email != "" && req.Email != fullUser.Email {
		existing, err := ctrl.userStore.GetUserByEmail(req.Email)
		if err == nil && existing != nil {
			c.JSON(409, auth.H{"error": "email already exists"})
			return
		}
		fullUser.Email = req.Email
	}

	if req.FullName != "" {
		fullUser.FullName = &req.FullName
	}

	if err := ctrl.userStore.UpdateUser(fullUser); err != nil {
		c.JSON(500, auth.H{"error": "failed to update user"})
		return
	}

	logger.Info("user updated", "username", fullUser.Username)
	c.JSON(200, auth.H{
		"message": "user updated successfully",
		"user": schema.UserResponse{
			ID:        fullUser.ID,
			Username:  fullUser.Username,
			Email:     fullUser.Email,
			FullName:  strVal(fullUser.FullName),
			AvatarURL: strVal(fullUser.AvatarURL),
			Status:    string(fullUser.Status),
			IsAdmin:   fullUser.IsAdmin,
		},
	})
}

// @Summary Change password
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body schema.ChangePasswordRequest true "Current and new password"
// @Success 200 {object} schema.MessageResponse
// @Failure 400 {object} schema.AuthErrorResponse
// @Failure 401 {object} schema.AuthErrorResponse
// @Failure 404 {object} schema.AuthErrorResponse
// @Failure 500 {object} schema.AuthErrorResponse
// @Router /auth/change-password [post]
func (ctrl *AuthController) ChangePassword(ctx context.Context, c *app.RequestContext) {
	user := auth.GetCurrentUser(c)
	if user == nil {
		c.JSON(401, auth.H{"error": "not authenticated"})
		return
	}

	var req schema.ChangePasswordRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, auth.H{"error": "invalid request body"})
		return
	}

	fullUser, err := ctrl.userStore.GetUserByID(user.ID)
	if err != nil {
		c.JSON(404, auth.H{"error": "user not found"})
		return
	}

	if !fullUser.VerifyPassword(req.CurrentPassword) {
		c.JSON(400, auth.H{"error": "current password is incorrect"})
		return
	}

	if err := fullUser.SetPassword(req.NewPassword); err != nil {
		c.JSON(500, auth.H{"error": "failed to process password"})
		return
	}

	if err := ctrl.userStore.UpdateUser(fullUser); err != nil {
		c.JSON(500, auth.H{"error": "failed to update password"})
		return
	}

	logger.Info("password changed", "username", fullUser.Username)
	c.JSON(200, auth.H{"message": "password changed successfully"})
}

func (ctrl *AuthController) ListUsers(ctx context.Context, c *app.RequestContext) {
	users, total, err := ctrl.userStore.ListUsers(1, 100)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	type userResp struct {
		ID          int64  `json:"id"`
		Username    string `json:"username"`
		Email       string `json:"email"`
		FullName    string `json:"full_name"`
		Status      string `json:"status"`
		IsSuperuser bool   `json:"is_superuser"`
		IsAdmin     bool   `json:"is_admin"`
	}
	var list []userResp
	for _, u := range users {
		fn := ""
		if u.FullName != nil {
			fn = *u.FullName
		}
		list = append(list, userResp{
			ID:          u.ID,
			Username:    u.Username,
			Email:       u.Email,
			FullName:    fn,
			Status:      string(u.Status),
			IsSuperuser: u.IsSuperuser,
			IsAdmin:     u.IsAdmin,
		})
	}
	c.JSON(consts.StatusOK, schema.SuccessResponse(map[string]any{
		"list":  list,
		"total": total,
	}))
}

func strVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
