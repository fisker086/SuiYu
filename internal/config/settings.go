package config

import (
	"os"
	"strconv"
	"strings"
)

const defaultJWTSecretWarning = "change-this-to-a-random-secret-key-in-production"

type Settings struct {
	ServerPort int

	ModelType string

	OpenAIAPIKey  string
	OpenAIModel   string
	OpenAIBaseURL string
	OpenAIAzure   bool

	ArkAPIKey  string
	ArkModel   string
	ArkBaseURL string

	LangfusePublicKey string
	LangfuseSecretKey string
	LangfuseHost      string

	SkillsDir string

	DatabaseURL        string
	EmbeddingDimension int
	EmbeddingAPIKey    string
	EmbeddingModel     string
	EmbeddingBaseURL   string

	// MemoryProvider: pgvector (embedding + pgvector storage) | none — swap later via new memory.Provider impl (e.g. mem0 client).
	MemoryProvider     string
	MemoryRetrieveTopK int

	JWTSecretKey string

	// AdminWhitelist is a comma-separated list of usernames who are platform admins
	AdminWhitelist string

	// GitHubRepo is the GitHub repository for version checks (e.g. "owner/repo")
	GitHubRepo string

	// DisableLoginCaptcha skips numeric captcha on POST /auth/login (dev/automation only; keep false in production).
	DisableLoginCaptcha bool

	// AuthType is the login method: password | lark | dingtalk | wecom | telegram
	AuthType string

	// SSO* OAuth credentials for the active provider (AUTH_TYPE). Same variable names for all platforms;
	// meaning depends on AUTH_TYPE (e.g. Lark uses Feishu open-apis; future DingTalk uses its endpoints).
	SSOAppID                string
	SSOAppSecret            string
	SSORedirectURI          string // backend callback, e.g. .../api/v1/auth/sso/lark/callback
	SSOOpenAPIBase          string // IdP API base (e.g. Lark: SSO_OPEN_API_BASE); empty → client default (feishu.cn)
	SSOOAuthSuccessURL      string // browser redirect after login; query appends tokens
	SSOCookieDomain         string // optional Set-Cookie Domain
	SSOAuthEnforceWhitelist bool   // if true, only existing DB users or SSO admin emails may sign in
	SSOAdminEmails          string // comma-separated emails that receive is_admin on first SSO upsert

	// UploadDir is the directory for uploaded files, relative to project root
	UploadDir string

	// SandboxType is the sandbox type: docker | kubernetes | none (default: docker)
	SandboxType string

	// SandboxDockerImages is a JSON map of language to docker image, e.g. {"python": "python:3.11-slim", "javascript": "node:22-slim"}
	SandboxDockerImages string

	// SandboxTimeoutSeconds is the timeout for sandbox execution (default: 30)
	SandboxTimeoutSeconds int

	// SandboxMemoryLimitMB is the memory limit in MB for sandbox (default: 256)
	SandboxMemoryLimitMB int

	// SandboxCPUPercent is the CPU limit percent for sandbox (default: 50)
	SandboxCPUPercent int
}

func Load() *Settings {
	return &Settings{
		ServerPort: getEnvInt("SERVER_PORT", 8080),

		ModelType: getEnv("MODEL_TYPE", "openai"),

		OpenAIAPIKey:  getEnv("OPENAI_API_KEY", ""),
		OpenAIModel:   getEnv("OPENAI_MODEL", "gpt-4o-mini"),
		OpenAIBaseURL: getEnv("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		OpenAIAzure:   getEnvBool("OPENAI_BY_AZURE", false),

		ArkAPIKey:  getEnv("ARK_API_KEY", ""),
		ArkModel:   getEnv("ARK_MODEL", ""),
		ArkBaseURL: getEnv("ARK_BASE_URL", ""),

		LangfusePublicKey: getEnv("LANGFUSE_PUBLIC_KEY", ""),
		LangfuseSecretKey: getEnv("LANGFUSE_SECRET_KEY", ""),
		LangfuseHost:      getEnv("LANGFUSE_HOST", "https://cloud.langfuse.com"),

		SkillsDir: getEnv("SKILLS_DIR", "./skills"),

		DatabaseURL:             getEnv("DATABASE_URL", ""),
		EmbeddingDimension:      getEnvInt("EMBEDDING_DIMENSION", 1536),
		EmbeddingAPIKey:         getEnv("EMBEDDING_API_KEY", ""),
		EmbeddingModel:          getEnv("EMBEDDING_MODEL", "text-embedding-3-small"),
		EmbeddingBaseURL:        getEnv("EMBEDDING_BASE_URL", "https://api.openai.com/v1"),
		MemoryProvider:          getEnv("MEMORY_PROVIDER", "pgvector"),
		MemoryRetrieveTopK:      getEnvInt("MEMORY_RETRIEVE_TOP_K", 8),
		GitHubRepo:              getEnv("GITHUB_REPO", ""),
		JWTSecretKey:            getEnv("JWT_SECRET_KEY", ""),
		AdminWhitelist:          getEnv("ADMIN_WHITELIST", ""),
		DisableLoginCaptcha:     getEnvBool("DISABLE_LOGIN_CAPTCHA", false),
		AuthType:                getEnv("AUTH_TYPE", "password"), // password | lark | dingtalk | wecom | telegram
		SSOAppID:                getEnv("SSO_APP_ID", ""),
		SSOAppSecret:            getEnv("SSO_APP_SECRET", ""),
		SSORedirectURI:          getEnv("SSO_REDIRECT_URI", ""),
		SSOOpenAPIBase:          getEnv("SSO_OPEN_API_BASE", ""),
		SSOOAuthSuccessURL:      getEnv("SSO_OAUTH_SUCCESS_REDIRECT", ""),
		SSOCookieDomain:         getEnv("SSO_COOKIE_DOMAIN", ""),
		SSOAuthEnforceWhitelist: getEnvBool("SSO_AUTH_ENFORCE_WHITELIST", false),
		SSOAdminEmails:          getEnv("SSO_ADMIN_EMAILS", ""),
		UploadDir:               getEnv("UPLOAD_DIR", "uploads"),

		SandboxType:            getEnv("SANDBOX_TYPE", "docker"),
		SandboxDockerImages:    getEnv("SANDBOX_DOCKER_IMAGES", `{"python": "python:3.11-slim", "javascript": "node:22-slim"}`),
		SandboxTimeoutSeconds:  getEnvInt("SANDBOX_TIMEOUT_SECONDS", 30),
		SandboxMemoryLimitMB:   getEnvInt("SANDBOX_MEMORY_LIMIT_MB", 256),
		SandboxCPUPercent:      getEnvInt("SANDBOX_CPU_PERCENT", 50),
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if val := os.Getenv(key); val != "" {
		if boolVal, err := strconv.ParseBool(val); err == nil {
			return boolVal
		}
	}
	return defaultVal
}

// EffectiveChatModel returns the model name used by the global chat client (aligned with cmd/server newChatModel).
func (s *Settings) EffectiveChatModel() string {
	if s == nil {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(s.ModelType)) {
	case "ark":
		if v := strings.TrimSpace(s.ArkModel); v != "" {
			return v
		}
	}
	if v := strings.TrimSpace(s.OpenAIModel); v != "" {
		return v
	}
	if v := strings.TrimSpace(s.ArkModel); v != "" {
		return v
	}
	return ""
}
