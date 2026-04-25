package skills

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
)

const toolBrowser = "builtin_browser"

// NewBuiltinBrowserTool registers the browser tool for agents that bind builtin_skill.browser_client.
// Execution is client-side only: Tauri launches a visible Chrome/Chromium (CDP), not server HTTP.
func NewBuiltinBrowserTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name: toolBrowser,
			Desc: "AI TaskMeta desktop only: drives a visible Chrome window via DevTools (CDP). Navigate, click, type, screenshot, extract text/HTML for the model. Session is isolated from your normal browser. If the user closed the window, call again to reopen. Not on this API server — use `builtin_http_client` for server-side HTTP fetch. IMPORTANT: the tool name is builtin_browser; do not rewrite it as browser_open_url/browser_search/browser_stop_recording in user-facing answers.",
			Extra: map[string]any{"execution_mode": "client"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation": {Type: einoschema.String, Desc: "open|goto|visit|fetch|text|html|click|type|press|screenshot|scroll|wait|eval|reload|back|close (https/allowed localhost http; see skill)", Required: true},
				"url":       {Type: einoschema.String, Desc: "URL for navigate ops (or target)", Required: false},
				"selector":  {Type: einoschema.String, Desc: "CSS selector for click/type/scroll", Required: false},
				"text":      {Type: einoschema.String, Desc: "Text for type/fill or key name for press", Required: false},
				"path":      {Type: einoschema.String, Desc: "Screenshot save path (optional)", Required: false},
				"js":        {Type: einoschema.String, Desc: "JavaScript for eval", Required: false},
				"timeout_ms": {Type: einoschema.Integer, Desc: "Optional. Max wait for elements (default 120000, max 180000). Raise on slow pages or flaky selectors.", Required: false},
				"nav_wait_ms": {Type: einoschema.Integer, Desc: "Optional. Milliseconds to sleep after navigate and after click/type (default 5000, max 60000). Raise if SPA loads slowly.", Required: false},
			}),
		},
		execBuiltinBrowserStub,
	)
}

func execBuiltinBrowserStub(_ context.Context, _ map[string]any) (string, error) {
	return "", fmt.Errorf("builtin_browser runs only on the AI TaskMeta desktop client (visible Chrome automation); start the desktop app and use a session with client-side tool execution")
}
