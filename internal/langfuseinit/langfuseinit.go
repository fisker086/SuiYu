// Package langfuseinit loads .env and registers Eino Langfuse callbacks when keys are present.
//
// 在 .env 或环境中设置：
//
//	LANGFUSE_PUBLIC_KEY / LANGFUSE_SECRET_KEY — 必填才会启用
//	LANGFUSE_HOST — 可选，默认 https://cloud.langfuse.com
//	LANGFUSE_ENABLED — 可选，设为 0|false|no 时强制关闭（即使配置了 key）
//
// 进程退出前应调用返回的 flush，确保 Trace 上报完成。
//
// eino-ext 回调创建根 Trace 时默认不写 Input、且易出 Unnamed：Init 会为 Handler 设默认 Name；
// 业务侧应对每次请求调用 WithUserQuery，把用户问题放进 Trace 的 metadata（SDK 当前不填根级 Input 字段）。
package langfuseinit

import (
	"context"
	"os"
	"strings"

	"github.com/cloudwego/eino-ext/callbacks/langfuse"
	"github.com/cloudwego/eino/callbacks"
)

// Init loads .env from the current directory (if present), then registers the global Langfuse handler
// when LANGFUSE_PUBLIC_KEY and LANGFUSE_SECRET_KEY are both non-empty.
// Returns a flush function that must be called before process exit (e.g. defer in main), and ok if tracing is on.
func Init() (flush func(), ok bool) {
	publicKey := strings.TrimSpace(os.Getenv("LANGFUSE_PUBLIC_KEY"))
	secretKey := strings.TrimSpace(os.Getenv("LANGFUSE_SECRET_KEY"))
	host := strings.TrimSpace(os.Getenv("LANGFUSE_HOST"))
	if host == "" {
		host = "https://cloud.langfuse.com"
	}

	if publicKey == "" || secretKey == "" {
		return func() {}, false
	}

	cfg := &langfuse.Config{
		Host:      host,
		PublicKey: publicKey,
		SecretKey: secretKey,
		Name:      "sya",
	}
	if n := strings.TrimSpace(os.Getenv("LANGFUSE_TRACE_NAME")); n != "" {
		cfg.Name = n
	}

	h, flusher := langfuse.NewLangfuseHandler(cfg)
	callbacks.AppendGlobalHandlers(h)
	return flusher, true
}

const maxUserQueryMetaLen = 8000

// WithUserQuery attaches Langfuse trace options to ctx so the root trace gets a clear name and
// the user message appears under trace metadata (input field is not set by eino-ext SDK).
func WithUserQuery(ctx context.Context, traceName, userQuery string) context.Context {
	name := strings.TrimSpace(traceName)
	if name == "" {
		name = "test"
	}
	q := strings.TrimSpace(userQuery)
	if len(q) > maxUserQueryMetaLen {
		q = q[:maxUserQueryMetaLen] + "…"
	}
	return langfuse.SetTrace(ctx,
		langfuse.WithName(name),
		langfuse.WithMetadata(map[string]string{
			"user_query": q,
		}),
		langfuse.WithEnvironment("qa"),
		langfuse.WithRelease("haha"),
		langfuse.WithPublic(true),
		langfuse.WithTags("a=1"),
	)
}
