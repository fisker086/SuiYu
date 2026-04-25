package larkbot

import (
	"strings"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	"github.com/fisk086/sya/internal/schema"
)

// OpenAPIDomainFromIMConfig returns the Open Platform base URL used for HTTP + long-connection WS.
// Default is Feishu China (open.feishu.cn). International Lark apps must use intl / open.larksuite.com
// or error 1000040351 (Incorrect domain name) may occur.
func OpenAPIDomainFromIMConfig(im schema.IMConfig) string {
	if d := strings.TrimSpace(im.LarkOpenDomain); d != "" {
		return strings.TrimRight(d, "/")
	}
	switch strings.ToLower(strings.TrimSpace(im.LarkRegion)) {
	case "intl", "larksuite", "global":
		return lark.LarkBaseUrl
	default:
		return lark.FeishuBaseUrl
	}
}

func openAPIDomainFromBot(cfg *BotConfig) string {
	if cfg == nil {
		return lark.FeishuBaseUrl
	}
	if d := strings.TrimSpace(cfg.OpenAPIDomain); d != "" {
		return strings.TrimRight(d, "/")
	}
	return lark.FeishuBaseUrl
}
