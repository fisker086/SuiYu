package storage

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// SessionTitleMaxRunes is the max length for auto-generated session titles from the first user message.
const SessionTitleMaxRunes = 48

// Leading "[图片×N] " / "[文件×N] " / "[图片×N 文件×M] " from userTextForMemory; omit from sidebar titles.
var sessionTitleAttachmentPrefix = regexp.MustCompile(`^\[(?:图片×\d+|文件×\d+)(?:\s+(?:图片×\d+|文件×\d+))?\]\s*`)

// SessionTitleFromFirstMessage builds a short display title from the first user message (whitespace normalized, rune-safe).
func SessionTitleFromFirstMessage(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	content = sessionTitleAttachmentPrefix.ReplaceAllString(content, "")
	content = strings.TrimSpace(content)
	content = strings.Join(strings.Fields(content), " ")
	if content == "" {
		return ""
	}
	if utf8.RuneCountInString(content) <= SessionTitleMaxRunes {
		return content
	}
	rs := []rune(content)
	return string(rs[:SessionTitleMaxRunes]) + "…"
}
