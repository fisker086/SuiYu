package skills

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
)

const toolVisibleBrowser = "builtin_visible_browser"

func NewBuiltinVisibleBrowserTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolVisibleBrowser,
			Desc:  "Visible browser automation - opens a Chrome window so user can see each step. Use for multi-step workflows like invoice reimbursement: init, goto URL, click elements, type text, upload files. Supports pause/resume for login.",
			Extra: map[string]any{"execution_mode": "client"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation": {Type: einoschema.String, Desc: "init/start, goto, click, type, fill, screenshot, html, text, title, url, wait, upload, close", Required: true},
				"url":       {Type: einoschema.String, Desc: "URL for goto/navigate operation", Required: false},
				"selector":  {Type: einoschema.String, Desc: "CSS selector for click/type/text operations", Required: false},
				"text":      {Type: einoschema.String, Desc: "Text to type into element", Required: false},
				"path":      {Type: einoschema.String, Desc: "File path for screenshot save or upload", Required: false},
				"seconds":   {Type: einoschema.Integer, Desc: "Seconds to wait", Required: false},
			}),
		},
		execBuiltinVisibleBrowserStub,
	)
}

func execBuiltinVisibleBrowserStub(_ context.Context, _ map[string]any) (string, error) {
	return "", fmt.Errorf("builtin_visible_browser is not available in this build; use builtin_browser (desktop visible Chrome/CDP) or builtin_http_client, or operate manually in a browser")
}
