package agent

import (
	"strings"

	einoschema "github.com/cloudwego/eino/schema"
)

// ensureOpenAICompatibleMessageContent mutates messages so JSON sent to OpenAI-compatible APIs
// always includes a non-empty "content" field where applicable.
//
// github.com/meguminnnnnnnnn/go-openai ChatCompletionMessage uses `json:"content,omitempty"` for
// the text Content field. Empty string is omitted, producing e.g. {"role":"system"} or
// {"role":"assistant","tool_calls":[...]} with no "content" key. LiteLLM's Vertex/Gemini
// transformation (_transform_system_message) does message["content"] and raises KeyError: 'content'.
//
// Multimodal messages (UserInputMultiContent / AssistantGenMultiContent / MultiContent) use a
// different MarshalJSON branch and are left unchanged.
func ensureOpenAICompatibleMessageContent(msgs []*einoschema.Message) {
	for _, m := range msgs {
		if m == nil {
			continue
		}
		if len(m.UserInputMultiContent) > 0 || len(m.AssistantGenMultiContent) > 0 || len(m.MultiContent) > 0 {
			continue
		}
		if strings.TrimSpace(m.Content) != "" {
			continue
		}
		m.Content = " "
	}
}
