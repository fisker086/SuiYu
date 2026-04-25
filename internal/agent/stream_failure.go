package agent

import (
	"strings"
)

// User-visible assistant lines when the model stream cannot complete (no raw errors to the client).
const (
	StreamFailureMessageUpstreamBlocked = "抱歉，模型服务暂时无法连接（可能被防护策略拦截或网络异常）。请稍后重试，或检查网络与 API 配置。"
	StreamFailureMessageInterrupted     = "抱歉，本次请求已中断或超时。请重试。"
	StreamFailureMessageGeneric         = "抱歉，本次回复未能生成。请稍后重试。"
	// StreamMessageTruncatedSuffix is appended when the client disconnected or the stream ended without [DONE].
	StreamMessageTruncatedSuffix = "\n\n—\n（本次回复未完整结束。）"
	// AssistantEmptyVisibleFallback is used when the model returns no visible text but the turn completed
	// without transport error (e.g. after client tool results). An empty SSE body would otherwise persist as
	// StreamFailureMessageGeneric ("抱歉，本次回复未能生成…") in chat history.
	AssistantEmptyVisibleFallback = "已根据工具结果完成本轮处理。若需要更详细的结论，请再发送一条消息或重试。"
	// PlanExecuteSynthesisEmptyFallback is used when plan-and-execute final synthesis returns empty content
	// but all steps completed without error.
	PlanExecuteSynthesisEmptyFallback = "任务执行完成，所有步骤已成功执行。"
)

// VisibleAssistantOrFallback returns trimmed primary when non-empty; otherwise fallback.
func VisibleAssistantOrFallback(primary, fallback string) string {
	if t := strings.TrimSpace(primary); t != "" {
		return t
	}
	if strings.TrimSpace(fallback) != "" {
		return fallback
	}
	return AssistantEmptyVisibleFallback
}

// UserVisibleStreamFailure maps an internal error to a safe, user-facing line for SSE + persistence.
func UserVisibleStreamFailure(err error) string {
	if err == nil {
		return StreamFailureMessageGeneric
	}
	s := strings.ToLower(err.Error())
	switch {
	case strings.Contains(s, "403") || strings.Contains(s, "html instead of json") ||
		strings.Contains(s, "invalid character '<'") || strings.Contains(s, "<!doctype"):
		return StreamFailureMessageUpstreamBlocked
	case strings.Contains(s, "context canceled"):
		return StreamFailureMessageInterrupted
	case strings.Contains(s, "timeout") || strings.Contains(s, "deadline exceeded"):
		return StreamFailureMessageInterrupted
	default:
		return StreamFailureMessageGeneric
	}
}

// IsSyntheticStreamFailureAssistant is true when the assistant text is our standard failure placeholder
// or a truncation suffix, so callers can skip embedding-based profile updates.
func IsSyntheticStreamFailureAssistant(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}
	if strings.Contains(s, "（本次回复未完整结束。）") {
		return true
	}
	switch s {
	case StreamFailureMessageUpstreamBlocked, StreamFailureMessageInterrupted, StreamFailureMessageGeneric:
		return true
	default:
		return false
	}
}
