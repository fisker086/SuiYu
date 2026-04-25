package skills

import (
	"fmt"
	"net"
	"strings"
)

func hostLooksUnsafe(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" || host == "localhost" {
		return true
	}
	if strings.HasPrefix(host, "[") {
		return true
	}
	h := host
	if i := strings.LastIndex(host, ":"); i > 0 && !strings.Contains(host, "]") {
		h = host[:i]
	}
	if ip := net.ParseIP(h); ip != nil {
		return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()
	}
	return false
}

func strArg(in map[string]any, keys ...string) string {
	for _, k := range keys {
		v, ok := in[k]
		if !ok || v == nil {
			continue
		}
		switch s := v.(type) {
		case string:
			return s
		default:
			return fmt.Sprint(s)
		}
	}
	return ""
}

func extractByPath(data any, key string) (any, error) {
	fields := strings.Split(key, ".")
	current := data
	for _, field := range fields {
		switch c := current.(type) {
		case map[string]any:
			v, ok := c[field]
			if !ok {
				return nil, fmt.Errorf("key not found: %s", field)
			}
			current = v
		default:
			return nil, fmt.Errorf("not an object at %s", field)
		}
	}
	return current, nil
}
