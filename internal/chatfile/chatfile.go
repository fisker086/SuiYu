// Package chatfile extracts text from chat-uploaded documents and formats injection blocks for the LLM.
package chatfile

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/ledongthuc/pdf"
)

// AttachedContentMarker begins server-injected file text. It is stripped before persisting user turns to agent_memory.
const AttachedContentMarker = "\n\n---\n[Attached file contents for model context]\n"

// chatAttachedHTTPHintText is prepended to the agent system prompt when uploads are present so the model
// does not redundantly call builtin_http_client on URLs that already appear in the injected message body.
const chatAttachedHTTPHintText = "（系统提示）用户已上传文件且正文已在消息里时，不要对文中 URL 重复使用 builtin_http_client，除非用户明确要求打开链接。"

// ChatAttachedHTTPHint returns a system-prompt fragment when the user attached files and the message
// is expected to include that body (injected block and/or file_urls). Returns empty when the rule does not apply.
func ChatAttachedHTTPHint(message string, fileURLs []string) string {
	if !messageHasAttachedContent(message, fileURLs) {
		return ""
	}
	return chatAttachedHTTPHintText
}

func messageHasAttachedContent(message string, fileURLs []string) bool {
	if strings.Contains(message, AttachedContentMarker) {
		return true
	}
	for _, u := range fileURLs {
		if strings.TrimSpace(u) != "" {
			return true
		}
	}
	return false
}

// PrependMemContext prepends ChatAttachedHTTPHint when applicable, before other memory/system prefixes.
func PrependMemContext(message string, fileURLs []string, memContext string) string {
	h := ChatAttachedHTTPHint(message, fileURLs)
	if h == "" {
		return memContext
	}
	if memContext == "" {
		return h
	}
	return h + "\n\n" + memContext
}

// StripAttachedContentForStorage removes the injected block so history/embeddings do not store full document text.
func StripAttachedContentForStorage(msg string) string {
	i := strings.Index(msg, AttachedContentMarker)
	if i < 0 {
		return msg
	}
	return strings.TrimSpace(msg[:i])
}

// ExtractText returns UTF-8 text for supported upload extensions (.txt .md .json .pdf).
func ExtractText(ext string, data []byte) (string, error) {
	ext = strings.ToLower(strings.TrimSpace(ext))
	switch ext {
	case ".txt", ".md", ".json":
		return normalizeTextBytes(data), nil
	case ".pdf":
		return extractPDFPlain(data)
	default:
		return "", fmt.Errorf("unsupported file type %s", ext)
	}
}

func normalizeTextBytes(data []byte) string {
	s := string(data)
	s = strings.ToValidUTF8(s, "\uFFFD")
	return strings.TrimSpace(s)
}

func extractPDFPlain(data []byte) (string, error) {
	if len(data) == 0 {
		return "", nil
	}
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("pdf open: %w", err)
	}
	rd, err := r.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("pdf plain text: %w", err)
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(rd); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

// TruncateRunes trims s to at most maxRunes Unicode code points, appending a notice if truncated.
func TruncateRunes(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	const note = "\n\n[... truncated for model context ...]"
	cut := maxRunes - utf8.RuneCountInString(note)
	if cut < 1 {
		cut = maxRunes
	}
	return string(runes[:cut]) + note
}
