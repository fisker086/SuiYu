package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	einoschema "github.com/cloudwego/eino/schema"
)

// streamStaticTextAsSSE splits a full reply into small SSE events so the UI can update progressively.
func streamStaticTextAsSSE(w io.Writer, text string) error {
	const chunk = 10
	runes := []rune(text)
	for i := 0; i < len(runes); i += chunk {
		j := i + chunk
		if j > len(runes) {
			j = len(runes)
		}
		if err := writeSSEJSON(w, string(runes[i:j])); err != nil {
			return err
		}
	}
	return nil
}

type sseTokenPayload struct {
	Content string `json:"content"`
}

func writeSSEJSON(w io.Writer, content string) error {
	if content == "" {
		return nil
	}
	b, err := json.Marshal(sseTokenPayload{Content: content})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", b)
	return err
}

func writeSSEJSONEvent(w io.Writer, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", b)
	return err
}

// assistantChunkText merges streaming assistant text + optional reasoning (same as plain LLM / ADK paths).
func assistantChunkText(m *einoschema.Message) string {
	if m == nil {
		return ""
	}
	out := m.Content
	if rc := strings.TrimSpace(m.ReasoningContent); rc != "" {
		if strings.TrimSpace(out) != "" {
			return m.ReasoningContent + "\n" + out
		}
		return m.ReasoningContent
	}
	return out
}
