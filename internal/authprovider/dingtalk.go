package authprovider

// 钉钉 OAuth 实现占位。
//
// 当前路由仍由 factory.go 中的 NewStubProvider("dingtalk") 响应（HTTP 501）。
// 实现时：在本文件增加 DingTalkProvider，实现 SSOProvider 接口，
// 并在 factory.go 的 DefaultProviders 里用 NewDingTalkProvider(...) 替换 stub。
//
// 可参考 lark.go + internal/larkauth，并单独增加 internal/dingtalkauth（或等价包）封装厂商 HTTP。

const dingTalkSSOImplementationPending = true

var _ = dingTalkSSOImplementationPending
