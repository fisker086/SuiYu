package chathistory

import (
	"strings"

	einoschema "github.com/cloudwego/eino/schema"
	"github.com/fisk086/sya/internal/schema"
)

// Placeholders for assistant rows persisted with empty content (e.g. client_tool handoff before final_answer).
const (
	EmptyAssistantWithStepsPlaceholder = "（无文字回复；详见侧栏思考过程）"
	EmptyAssistantPlaceholder          = "（无文字回复）"
)

// IncludeInLLMContext is false for assistant messages with no text (empty content confuses some providers).
// User rows with only attachments are kept.
func IncludeInLLMContext(m schema.ChatHistoryMessage) bool {
	switch m.Role {
	case "assistant":
		return strings.TrimSpace(m.Content) != ""
	case "user":
		if strings.TrimSpace(m.Content) != "" {
			return true
		}
		return len(m.ImageURLs) > 0 || len(m.FileURLs) > 0
	default:
		return strings.TrimSpace(m.Content) != ""
	}
}

// ToEinoMessages converts stored turns to chat model messages, omitting rows that must not be sent to the LLM.
func ToEinoMessages(msgs []schema.ChatHistoryMessage) []*einoschema.Message {
	var out []*einoschema.Message
	for _, m := range msgs {
		if !IncludeInLLMContext(m) {
			continue
		}
		role := einoschema.User
		if m.Role == "assistant" {
			role = einoschema.Assistant
		}
		out = append(out, &einoschema.Message{
			Role:    role,
			Content: m.Content,
		})
	}
	return out
}

// NormalizeEmptyAssistantForAPI fills placeholder text so clients never render a blank assistant bubble
// while still returning react_steps on the same row.
func NormalizeEmptyAssistantForAPI(msgs []schema.ChatHistoryMessage) {
	for i := range msgs {
		if msgs[i].Role != "assistant" {
			continue
		}
		if strings.TrimSpace(msgs[i].Content) != "" {
			continue
		}
		if len(msgs[i].ReactSteps) > 0 {
			msgs[i].Content = EmptyAssistantWithStepsPlaceholder
		} else {
			msgs[i].Content = EmptyAssistantPlaceholder
		}
	}
}
