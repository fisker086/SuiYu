package controller

import "strings"

// NormalizeClientTypeFromUserAgent reconciles JSON client_type with the User-Agent header.
//
// We do NOT downgrade client_type=desktop → web based on UA: Tauri/Wry/WebView2/WKWebView often use a
// stock Chrome/Safari-like User-Agent with no "tauri" or "wry" substring, so that rule misclassified
// real desktop clients as web. Mis-sent desktop from a browser tab should be fixed in the client
// (e.g. isTauri() for AgentSphere).
//
// Rules (all case-insensitive):
//   - web + UA containing tauri / wry / electron/ → desktop
//   - empty + same shell hints → desktop
func NormalizeClientTypeFromUserAgent(reqClientType, userAgent string) string {
	ct := strings.ToLower(strings.TrimSpace(reqClientType))
	ua := strings.TrimSpace(userAgent)
	if ua == "" {
		return strings.TrimSpace(reqClientType)
	}
	ual := strings.ToLower(ua)

	desktopShell := strings.Contains(ual, "tauri") ||
		strings.Contains(ual, "wry") ||
		strings.Contains(ual, "electron/")

	switch ct {
	case "web":
		if desktopShell {
			return "desktop"
		}
	case "":
		if desktopShell {
			return "desktop"
		}
	}
	return strings.TrimSpace(reqClientType)
}
