package service

import (
	"encoding/json"
	"strings"
)

// parseSSEReactStepPayloads collects JSON objects from SSE lines that carry ReAct (type) or ADK (event_type)
// metadata, for persistence in agent_memory.extra.react_steps. The frontend hydrates these into ChatReactStep.
func parseSSEReactStepPayloads(raw []byte) []map[string]any {
	var out []map[string]any
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		chunk := strings.TrimPrefix(line, "data: ")
		if strings.TrimSpace(chunk) == "[DONE]" {
			continue
		}
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(chunk), &m); err != nil || m == nil {
			continue
		}
		if t, ok := m["type"].(string); ok && strings.TrimSpace(t) != "" {
			out = append(out, m)
			continue
		}
		if et, ok := m["event_type"].(string); ok && strings.TrimSpace(et) != "" {
			out = append(out, m)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
