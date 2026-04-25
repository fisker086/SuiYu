package schema

// AuthErrorResponse is the common JSON body for auth error responses: {"error":"..."}.
type AuthErrorResponse struct {
	Error string `json:"error"`
}

// RegisterSuccessResponse is returned by POST /auth/register on success.
type RegisterSuccessResponse struct {
	Message string       `json:"message"`
	User    UserResponse `json:"user"`
}

// TokenPairResponse is returned by POST /auth/refresh (tokens only, no user profile).
type TokenPairResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

// GetMeResponse is returned by GET /auth/me.
type GetMeResponse struct {
	User UserResponse `json:"user"`
}

// ProfileUpdateResponse is returned by PUT /auth/me.
type ProfileUpdateResponse struct {
	Message string       `json:"message"`
	User    UserResponse `json:"user"`
}

// MessageResponse is a simple {"message":"..."} success payload.
type MessageResponse struct {
	Message string `json:"message"`
}

type LoginRequest struct {
	Username     string `json:"username" binding:"required"`
	Password     string `json:"password" binding:"required"`
	CaptchaToken string `json:"captcha_token,omitempty"`
	CaptchaCode  string `json:"captcha_code,omitempty"`
}

type RegisterRequest struct {
	Username     string `json:"username" binding:"required"`
	Email        string `json:"email" binding:"required"`
	Password     string `json:"password" binding:"required,min=6"`
	FullName     string `json:"full_name,omitempty"`
	CaptchaToken string `json:"captcha_token,omitempty"`
	CaptchaCode  string `json:"captcha_code,omitempty"`
}

type TokenResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	TokenType    string       `json:"token_type"`
	ExpiresIn    int64        `json:"expires_in"`
	User         UserResponse `json:"user"`
}

type UserResponse struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	FullName  string `json:"full_name,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
	Status    string `json:"status"`
	IsAdmin   bool   `json:"is_admin"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=6"`
}

type UpdateProfileRequest struct {
	Email    string `json:"email,omitempty"`
	FullName string `json:"full_name,omitempty"`
}
