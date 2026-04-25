package authprovider

// 企业微信 OAuth 实现占位。
//
// 当前路由仍由 factory.go 中的 NewStubProvider("wecom") 响应（HTTP 501）。
// 实现时：在本文件增加 WeComProvider，实现 SSOProvider 接口，
// 并在 factory.go 的 DefaultProviders 里用 NewWeComProvider(...) 替换 stub。
//
// 可参考 lark.go + internal/larkauth，并单独增加 internal/wecomauth（或等价包）封装厂商 HTTP。

const weComSSOImplementationPending = true

var _ = weComSSOImplementationPending
