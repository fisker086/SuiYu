package notify

import (
	"encoding/json"
	"strings"
	"unicode/utf8"
)

// WrapForLarkMarkdownFence wraps arbitrary text in a fenced ```text block so Feishu interactive card
// markdown does not fail on *, _, backticks, ---, or other stderr/tool output (Lark err 11311).
func WrapForLarkMarkdownFence(s string) string {
	s = strings.ToValidUTF8(s, "\uFFFD")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	// Prevent closing the outer fence early
	s = strings.ReplaceAll(s, "```", "``\u200b`")
	return "```text\n" + s + "\n```"
}

// Lark card body size limit (PATCH ~30KB; leave margin for header).
const larkCardMarkdownMaxRunes = 28000

// buildLarkInteractiveCardMap returns Feishu message card JSON 2.0 so markdown uses the full renderer
// (ATX headings #/##/###, inline code, etc.). Card 1.0 markdown only supports a subset, so those
// tokens appeared as literal text in the client.
func buildLarkInteractiveCardMap(title, markdownBody string) map[string]any {
	t := truncateRunes(strings.TrimSpace(title), 100)
	if t == "" {
		t = "通知"
	}
	md := normalizeLarkCardMarkdown(markdownBody)
	return map[string]any{
		"schema": "2.0",
		"config": map[string]any{
			"update_multi": true,
			"width_mode":   "fill",
		},
		"header": map[string]any{
			"template": "blue",
			"title": map[string]any{
				"tag":     "plain_text",
				"content": t,
			},
		},
		"body": map[string]any{
			"elements": []any{
				map[string]any{
					"tag":        "markdown",
					"element_id": "md_main",
					"content":    md,
				},
			},
		},
	}
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

// larkCardTitleAndMarkdown chooses header title and markdown body.
// extra.card_title overrides; otherwise first line becomes title when text is multi-line.
func larkCardTitleAndMarkdown(extra map[string]string, fullText string) (title, md string) {
	if t := strings.TrimSpace(extraVal(extra, "card_title")); t != "" {
		return truncateRunes(t, 100), strings.TrimSpace(fullText)
	}
	fullText = strings.TrimSpace(fullText)
	if fullText == "" {
		return "通知", ""
	}
	norm := strings.ReplaceAll(strings.ReplaceAll(fullText, "\r\n", "\n"), "\r", "\n")
	parts := strings.SplitN(norm, "\n", 2)
	if len(parts) < 2 {
		return "通知", norm
	}
	first := strings.TrimSpace(parts[0])
	rest := strings.TrimSpace(parts[1])
	if first == "" {
		return "通知", norm
	}
	return truncateRunes(first, 100), rest
}

func normalizeLarkCardMarkdown(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return " "
	}
	if utf8.RuneCountInString(s) <= larkCardMarkdownMaxRunes {
		return s
	}
	r := []rune(s)
	return string(r[:larkCardMarkdownMaxRunes]) + "\n\n…(truncated)"
}

// larkInteractiveContentString returns JSON string for im/v1/messages content field.
func larkInteractiveContentString(title, markdownBody string) string {
	m := buildLarkInteractiveCardMap(title, markdownBody)
	b, err := json.Marshal(m)
	if err != nil {
		return "{}"
	}
	return string(b)
}
