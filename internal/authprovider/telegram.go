package authprovider

// Telegram 登录占位（常见为 Login Widget / OAuth Bot，与飞书企业 OAuth 流程不同，但同样实现 SSOProvider）。
//
// 当前路由仍由 factory.go 中的 NewStubProvider("telegram") 响应（HTTP 501）。
// 实现时：在本文件增加 TelegramProvider，实现 SSOProvider 接口，
// 并在 factory.go 的 DefaultProviders 里用 NewTelegramProvider(...) 替换 stub。
//
// 可参考 lark.go 的分发与 JWT 签发；HTTP 细节可放在 internal/telegramauth（或等价包）。

const telegramSSOImplementationPending = true

var _ = telegramSSOImplementationPending
