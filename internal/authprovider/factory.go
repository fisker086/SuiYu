package authprovider

import (
	"github.com/fisk086/sya/internal/auth"
	"github.com/fisk086/sya/internal/config"
	"github.com/fisk086/sya/internal/storage"
)

// DefaultProviders returns all known SSO channels for registration order:
// Lark (real); DingTalk / WeCom / Telegram (stubs — see dingtalk.go / wecom.go / telegram.go).
func DefaultProviders(userStore storage.UserStore, jwtCfg auth.JWTConfig, settings *config.Settings) []SSOProvider {
	if userStore == nil {
		return nil
	}
	return []SSOProvider{
		NewLarkProvider(userStore, jwtCfg, settings),
		NewStubProvider("dingtalk"),
		NewStubProvider("wecom"),
		NewStubProvider("telegram"),
	}
}
