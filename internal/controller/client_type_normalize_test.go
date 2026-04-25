package controller

import "testing"

func TestNormalizeClientTypeFromUserAgent(t *testing.T) {
	chrome := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	tauri := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Tauri/2"
	electron := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Electron/28.0.0 Safari/537.36"

	tests := []struct {
		name   string
		bodyCT string
		ua     string
		want   string
	}{
		{"empty UA unchanged", "desktop", "", "desktop"},
		// Stock WebView UA (no tauri/wry in string) must stay desktop — same string as desktop Chrome.
		{"desktop stock chrome-like UA stays desktop", "desktop", chrome, "desktop"},
		{"desktop tauri UA stays desktop", "desktop", tauri, "desktop"},
		{"desktop wry UA stays desktop", "desktop", "Mozilla/5.0 wry/0.1", "desktop"},
		{"desktop electron UA stays desktop", "desktop", electron, "desktop"},
		{"web tauri UA to desktop", "web", tauri, "desktop"},
		{"web chrome UA unchanged", "web", chrome, "web"},
		{"empty tauri to desktop", "", tauri, "desktop"},
		{"empty chrome stays empty", "", chrome, ""},
		{"preserve unknown token", "mobile", chrome, "mobile"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeClientTypeFromUserAgent(tt.bodyCT, tt.ua)
			if got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}
