package controller

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fisk086/sya/internal/chatfile"
	"github.com/fisk086/sya/internal/schema"
)

// Per-file and total caps so a single upload cannot exhaust model context.
const (
	maxChatFileExtractRunes = 120_000
	maxChatMessageRunes     = 480_000
)

// appendChatFileContentsToMessage reads uploaded files (same paths as /chat/upload), extracts text,
// and appends to req.Message for the model. Strips any client-spoofed injection marker first.
// Requires authenticated chatUserID (uploads are per-user).
func (c *ChatController) appendChatFileContentsToMessage(req *schema.ChatRequest, chatUserID string) error {
	if req == nil || len(req.FileURLs) == 0 {
		return nil
	}
	if chatUserID == "" {
		return fmt.Errorf("file_urls require authentication")
	}
	req.Message = chatfile.StripAttachedContentForStorage(req.Message)

	base := filepath.Clean(c.uploadDir)
	var sb strings.Builder
	n := 0
	for _, raw := range req.FileURLs {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		rel, err := chatUploadRelPath(raw, chatUserID)
		if err != nil {
			return fmt.Errorf("file url: %w", err)
		}
		full := filepath.Join(base, rel)
		fullClean := filepath.Clean(full)
		if !strings.HasPrefix(fullClean, base+string(os.PathSeparator)) && fullClean != base {
			return fmt.Errorf("invalid file path")
		}
		data, err := os.ReadFile(fullClean)
		if err != nil {
			return fmt.Errorf("read uploaded file: %w", err)
		}
		ext := strings.ToLower(filepath.Ext(rel))
		text, err := chatfile.ExtractText(ext, data)
		if err != nil {
			return fmt.Errorf("extract file content: %w", err)
		}
		if strings.TrimSpace(text) == "" {
			text = "(no extractable text)"
		}
		text = chatfile.TruncateRunes(text, maxChatFileExtractRunes)
		if n == 0 {
			sb.WriteString(chatfile.AttachedContentMarker)
		} else {
			sb.WriteString("\n\n")
		}
		sb.WriteString("--- File: ")
		sb.WriteString(filepath.Base(rel))
		sb.WriteString(" ---\n")
		sb.WriteString(text)
		n++
	}
	if n == 0 {
		return nil
	}
	req.Message = strings.TrimSpace(req.Message) + sb.String()
	req.Message = chatfile.TruncateRunes(req.Message, maxChatMessageRunes)
	return nil
}
