package storage

import (
	"encoding/json"
	"strings"
)

// ChatHistoryAttachmentsFromExtraJSONB parses agent_memory.extra JSONB for GET /chat/sessions/:id/messages.
// Supports image_urls / file_urls as JSON arrays or a single JSON string; legacy image_url (string).
func ChatHistoryAttachmentsFromExtraJSONB(extraBytes []byte) (imageURLs []string, fileURLs []string) {
	if len(extraBytes) == 0 {
		return nil, nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(extraBytes, &m); err != nil || m == nil {
		return nil, nil
	}
	imageURLs = stringSliceFromJSONRaw(m["image_urls"])
	fileURLs = stringSliceFromJSONRaw(m["file_urls"])
	if raw, ok := m["image_url"]; ok && len(imageURLs) == 0 {
		var one string
		if err := json.Unmarshal(raw, &one); err == nil {
			one = strings.TrimSpace(one)
			if one != "" {
				imageURLs = []string{one}
			}
		}
	}
	return imageURLs, fileURLs
}

func stringSliceFromJSONRaw(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var s []string
	if err := json.Unmarshal(raw, &s); err == nil && s != nil {
		out := make([]string, 0, len(s))
		for _, u := range s {
			u = strings.TrimSpace(u)
			if u != "" {
				out = append(out, u)
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	}
	var one string
	if err := json.Unmarshal(raw, &one); err == nil {
		one = strings.TrimSpace(one)
		if one != "" {
			return []string{one}
		}
	}
	return nil
}

// ChatHistoryReactStepsFromExtraJSONB parses agent_memory.extra JSONB for react_steps (ReAct / ADK SSE payloads).
func ChatHistoryReactStepsFromExtraJSONB(extraBytes []byte) []map[string]any {
	if len(extraBytes) == 0 {
		return nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(extraBytes, &m); err != nil || m == nil {
		return nil
	}
	raw, ok := m["react_steps"]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var steps []map[string]any
	if err := json.Unmarshal(raw, &steps); err != nil {
		return nil
	}
	if len(steps) == 0 {
		return nil
	}
	return steps
}
