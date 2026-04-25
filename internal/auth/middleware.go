package auth

import (
	"context"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
)

type UserContextKey struct{}

func JWTMiddleware(cfg JWTConfig, getUser func(userID int64) (*User, error)) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		authHeader := string(c.GetHeader("Authorization"))
		if authHeader == "" {
			token := c.Query("access_token")
			if token == "" {
				c.AbortWithStatusJSON(401, H{"error": "missing auth"})
				return
			}
			authHeader = "Bearer " + token
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(401, H{"error": "invalid auth header"})
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := ParseToken(cfg, tokenString)
		if err != nil {
			c.AbortWithStatusJSON(401, H{"error": "invalid or expired token"})
			return
		}

		if claims.Type != "access" {
			c.AbortWithStatusJSON(401, H{"error": "invalid token type"})
			return
		}

		user, err := getUser(claims.UserID)
		if err != nil {
			c.AbortWithStatusJSON(401, H{"error": "user not found"})
			return
		}

		if user.Status != "active" {
			c.AbortWithStatusJSON(403, H{"error": "user account is not active"})
			return
		}

		c.Set("current_user", user)
		c.Next(ctx)
	}
}

func GetCurrentUser(c *app.RequestContext) *User {
	val, exists := c.Get("current_user")
	if !exists {
		return nil
	}
	user, ok := val.(*User)
	if !ok {
		return nil
	}
	return user
}

// OptionalJWTMiddleware parses Authorization: Bearer when present and sets current_user.
// Requests without a Bearer token continue without a user (handlers must not assume auth).
// Invalid or expired tokens return 401 so clients cannot silently fall back to unscoped lists.
func OptionalJWTMiddleware(cfg JWTConfig, getUser func(userID int64) (*User, error)) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		authHeader := string(c.GetHeader("Authorization"))
		if authHeader == "" {
			c.Next(ctx)
			return
		}
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.Next(ctx)
			return
		}
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := ParseToken(cfg, tokenString)
		if err != nil {
			c.AbortWithStatusJSON(401, H{"error": "invalid or expired token"})
			return
		}
		if claims.Type != "access" {
			c.AbortWithStatusJSON(401, H{"error": "invalid token type"})
			return
		}
		user, err := getUser(claims.UserID)
		if err != nil {
			c.AbortWithStatusJSON(401, H{"error": "user not found"})
			return
		}
		if user.Status != "active" {
			c.AbortWithStatusJSON(403, H{"error": "user account is not active"})
			return
		}
		c.Set("current_user", user)
		c.Next(ctx)
	}
}

type H map[string]any

type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Status   string `json:"status"`
	IsAdmin  bool   `json:"is_admin"`
}
