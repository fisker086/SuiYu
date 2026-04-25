package mcp

import "strings"

// HeadersFromConfig builds HTTP headers for MCP transport: config["headers"],
// plus optional bearer_token / api_key / token → Authorization: Bearer …
func HeadersFromConfig(cfg map[string]any) map[string]string {
	if cfg == nil {
		return nil
	}
	out := make(map[string]string)

	if raw, ok := cfg["headers"]; ok && raw != nil {
		switch v := raw.(type) {
		case map[string]string:
			for k, val := range v {
				if k != "" && val != "" {
					out[k] = val
				}
			}
		case map[string]any:
			for k, val := range v {
				if s, ok := val.(string); ok && k != "" && s != "" {
					out[k] = s
				}
			}
		}
	}

	bearer := ""
	for _, key := range []string{"bearer_token", "api_key", "token"} {
		if v, ok := cfg[key]; ok && v != nil {
			if s, ok := v.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" {
					bearer = s
					break
				}
			}
		}
	}
	if bearer != "" {
		if _, has := out["Authorization"]; !has {
			if _, has2 := out["authorization"]; !has2 {
				low := strings.ToLower(bearer)
				if strings.HasPrefix(low, "bearer ") {
					out["Authorization"] = bearer
				} else {
					out["Authorization"] = "Bearer " + bearer
				}
			}
		}
	}

	if len(out) == 0 {
		return nil
	}
	return out
}
