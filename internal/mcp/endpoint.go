package mcp

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fisk086/sya/internal/schema"
)

// NormalizeMCPTransport returns a canonical transport string for the MCP client.
func NormalizeMCPTransport(transport string) string {
	t := strings.ToLower(strings.TrimSpace(transport))
	switch t {
	case "stdio":
		return "stdio"
	case "streamable-http", "http", "streamable":
		return "streamable-http"
	case "sse", "":
		return "sse"
	default:
		return t
	}
}

// ResolveMCPConnection returns the transport and connection target (URL or shell line)
// to register with the MCP client. It infers stdio when the stored transport is still
// the default SSE family but auth JSON only defines command/args (common UI mistake).
func ResolveMCPConnection(cfg *schema.MCPConfig) (transport, target string, ok bool) {
	if cfg == nil {
		return "", "", false
	}
	if ep := strings.TrimSpace(cfg.Endpoint); ep != "" {
		return NormalizeMCPTransport(cfg.Transport), ep, true
	}

	t0 := NormalizeMCPTransport(cfg.Transport)
	maps := eachConfigMap(cfg)

	if t0 == "stdio" {
		if tr, tgt, ok := firstStdioInMaps(maps); ok {
			return tr, tgt, true
		}
		if tr, tgt, ok := firstURLInMaps(maps, "sse"); ok {
			return tr, tgt, true
		}
		return "", "", false
	}

	if tr, tgt, ok := firstURLInMaps(maps, t0); ok {
		return tr, tgt, true
	}
	if tr, tgt, ok := firstStdioInMaps(maps); ok {
		return tr, tgt, true
	}
	return "", "", false
}

func firstStdioInMaps(maps []map[string]any) (transport, target string, ok bool) {
	for _, m := range maps {
		if s := stdioCommandLineFromMap(m); s != "" {
			return "stdio", s, true
		}
	}
	return "", "", false
}

func firstURLInMaps(maps []map[string]any, httpTransport string) (transport, target string, ok bool) {
	for _, m := range maps {
		if s := urlFromConfigMap(m); s != "" {
			return httpTransport, s, true
		}
	}
	return "", "", false
}

// EffectiveConnectionTarget returns only the URL or shell snippet (backward compatible).
func EffectiveConnectionTarget(cfg *schema.MCPConfig) string {
	_, target, ok := ResolveMCPConnection(cfg)
	if !ok {
		return ""
	}
	return target
}

func serverBlocksFromRoot(root map[string]any) []map[string]any {
	var out []map[string]any
	for _, key := range []string{"mcpServers", "servers"} {
		if m, ok := root[key].(map[string]any); ok && len(m) > 0 {
			out = append(out, m)
		}
	}
	return out
}

func mapHasCommandOrCmd(m map[string]any) bool {
	if m == nil {
		return false
	}
	if _, ok := firstStringField(m, "command", "cmd"); ok {
		return true
	}
	return false
}

// eachConfigMap returns config subtrees to scan, in priority order.
func eachConfigMap(cfg *schema.MCPConfig) []map[string]any {
	var ms []map[string]any
	if cfg == nil || cfg.Config == nil {
		return ms
	}
	root := cfg.Config
	t0 := NormalizeMCPTransport(cfg.Transport)
	blocks := serverBlocksFromRoot(root)

	if cfg.Key != "" {
		for _, servers := range blocks {
			if entry, found := findMcpServerEntry(servers, cfg.Key); found && len(entry) > 0 {
				ms = append(ms, entry)
				break
			}
		}
	} else if t0 == "stdio" {
	outer:
		for _, servers := range blocks {
			for _, name := range sortedStringKeys(servers) {
				if entry, ok := servers[name].(map[string]any); ok && mapHasCommandOrCmd(entry) {
					ms = append(ms, entry)
					break outer
				}
			}
		}
	}

	ms = append(ms, root)
	if sm, ok := root["server"].(map[string]any); ok && len(sm) > 0 {
		ms = append(ms, sm)
	}

	for _, servers := range blocks {
		for name, v := range servers {
			if cfg.Key != "" && strings.EqualFold(name, cfg.Key) {
				continue
			}
			if entry, ok := v.(map[string]any); ok && len(entry) > 0 {
				ms = append(ms, entry)
			}
		}
	}
	return expandOneLevelNestedMaps(ms)
}

// expandOneLevelNestedMaps appends child objects that look like server entries
// (command/cmd or URL keys), e.g. {"my": {"command": "npx"}}.
func expandOneLevelNestedMaps(maps []map[string]any) []map[string]any {
	var out []map[string]any
	for _, m := range maps {
		if m == nil {
			continue
		}
		out = append(out, m)
		for _, v := range m {
			child, ok := v.(map[string]any)
			if !ok || len(child) == 0 {
				continue
			}
			if mapHasCommandOrCmd(child) || urlFromConfigMap(child) != "" {
				out = append(out, child)
			}
		}
	}
	return out
}

func findMcpServerEntry(servers map[string]any, key string) (map[string]any, bool) {
	if key == "" || servers == nil {
		return nil, false
	}
	for k, v := range servers {
		if strings.EqualFold(k, key) {
			if entry, ok := v.(map[string]any); ok {
				return entry, true
			}
		}
	}
	return nil, false
}

func sortedStringKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func urlFromConfigMap(m map[string]any) string {
	if m == nil {
		return ""
	}
	for _, k := range []string{"url", "endpoint", "server_url", "base_url", "sse_url", "mcp_url"} {
		if s, ok := stringFromAny(m[k]); ok {
			return s
		}
	}
	return ""
}

func stdioCommandLineFromMap(m map[string]any) string {
	if m == nil {
		return ""
	}
	cmd, ok := firstStringField(m, "command", "cmd")
	if !ok {
		return ""
	}
	args := argsFromMap(m)
	if len(args) == 0 {
		return shellQuotePOSIX(cmd)
	}
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, shellQuotePOSIX(cmd))
	for _, a := range args {
		parts = append(parts, shellQuotePOSIX(a))
	}
	return strings.Join(parts, " ")
}

func firstStringField(m map[string]any, keys ...string) (string, bool) {
	for _, k := range keys {
		if s, ok := stringFromAny(m[k]); ok {
			return s, true
		}
	}
	return "", false
}

func argsFromMap(m map[string]any) []string {
	a := stringSliceFromAny(m["args"])
	if len(a) == 0 {
		a = stringSliceFromAny(m["argv"])
	}
	return a
}

func stringFromAny(v any) (string, bool) {
	switch x := v.(type) {
	case string:
		s := strings.TrimSpace(x)
		return s, s != ""
	case float64:
		if x == float64(int64(x)) {
			s := fmt.Sprintf("%.0f", x)
			return s, s != ""
		}
		s := strings.TrimSpace(fmt.Sprint(x))
		return s, s != ""
	case int, int32, int64:
		s := strings.TrimSpace(fmt.Sprint(x))
		return s, s != ""
	case bool:
		return "", false
	default:
		return "", false
	}
}

func stringSliceFromAny(v any) []string {
	switch x := v.(type) {
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return nil
		}
		return strings.Fields(s)
	case []string:
		out := make([]string, 0, len(x))
		for _, s := range x {
			if t := strings.TrimSpace(s); t != "" {
				out = append(out, t)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(x))
		for _, e := range x {
			if s, ok := stringFromAny(e); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

// shellQuotePOSIX returns s wrapped for safe use inside sh -c (POSIX single-quote).
func shellQuotePOSIX(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
