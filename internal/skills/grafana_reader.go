package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
)

const toolGrafanaReader = "builtin_grafana_reader"

var allowedGrafanaOps = map[string]bool{
	"dashboards":  true,
	"dashboard":   true,
	"panels":      true,
	"alerts":      true,
	"datasources": true,
}

func execBuiltinGrafanaReader(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		op = "dashboards"
	}

	if !allowedGrafanaOps[op] {
		return "", fmt.Errorf("operation %q not allowed (read-only mode; allowed: %v)", op, allowedGrafanaOps)
	}

	grafanaURL := strArg(in, "grafana_url", "url", "endpoint")
	if grafanaURL == "" {
		grafanaURL = "http://localhost:3000"
	}
	grafanaURL = strings.TrimRight(grafanaURL, "/")

	apiKey := strArg(in, "api_key", "apikey", "token")

	var apiPath string
	switch op {
	case "dashboards":
		apiPath = "/api/search?type=dash-db"
	case "dashboard":
		uid := strArg(in, "dashboard_uid", "uid", "dashboard_id")
		if uid == "" {
			return "", fmt.Errorf("missing dashboard_uid")
		}
		apiPath = "/api/dashboards/uid/" + uid
	case "panels":
		uid := strArg(in, "dashboard_uid", "uid", "dashboard_id")
		if uid == "" {
			return "", fmt.Errorf("missing dashboard_uid")
		}
		apiPath = "/api/dashboards/uid/" + uid
	case "alerts":
		apiPath = "/api/alerts"
	case "datasources":
		apiPath = "/api/datasources"
	}

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", grafanaURL+apiPath, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("Failed to connect to Grafana at %s: %v", grafanaURL, err), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("Grafana returned HTTP %d: %s", resp.StatusCode, string(body)), nil
	}

	var result any
	if err := json.Unmarshal(body, &result); err != nil {
		return string(body), nil
	}

	pretty, _ := json.MarshalIndent(result, "", "  ")
	return fmt.Sprintf("grafana %s result:\n\n%s", op, string(pretty)), nil
}

func NewBuiltinGrafanaReaderTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolGrafanaReader,
			Desc:  "Read Grafana dashboards, panels, alerts, datasources. Read-only.",
			Extra: map[string]any{"execution_mode": "server"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation":     {Type: einoschema.String, Desc: "Operation: dashboards, dashboard, panels, alerts, datasources", Required: false},
				"grafana_url":   {Type: einoschema.String, Desc: "Grafana server URL (default: http://localhost:3000)", Required: false},
				"dashboard_uid": {Type: einoschema.String, Desc: "Dashboard UID (for dashboard/panels)", Required: false},
				"api_key":       {Type: einoschema.String, Desc: "Grafana API key", Required: false},
			}),
		},
		execBuiltinGrafanaReader,
	)
}
