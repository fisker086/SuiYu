package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
)

const toolPrometheusQuery = "builtin_prometheus_query"

var allowedPromOps = map[string]bool{
	"query":       true,
	"query_range": true,
	"alerts":      true,
	"targets":     true,
	"metrics":     true,
}

func buildPrometheusURL(baseURL, apiPath string, params map[string]string) (string, error) {
	base, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return "", fmt.Errorf("invalid prometheus_url %q: %w", baseURL, err)
	}

	ref, err := url.Parse(apiPath)
	if err != nil {
		return "", fmt.Errorf("invalid prometheus api path %q: %w", apiPath, err)
	}

	full := base.ResolveReference(ref)
	query := full.Query()
	for k, v := range params {
		if v != "" {
			query.Set(k, v)
		}
	}
	full.RawQuery = query.Encode()
	return full.String(), nil
}

func execBuiltinPrometheusQuery(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		op = "alerts"
	}

	if !allowedPromOps[op] {
		return "", fmt.Errorf("operation %q not allowed (read-only mode; allowed: %v)", op, allowedPromOps)
	}

	promURL := strArg(in, "prometheus_url", "url", "endpoint")
	if promURL == "" {
		promURL = "http://localhost:9090"
	}
	promURL = strings.TrimRight(promURL, "/")

	var apiPath string
	var params map[string]string

	switch op {
	case "query":
		q := strArg(in, "query", "q", "expr", "promql")
		if q == "" {
			return "", fmt.Errorf("missing query expression")
		}
		apiPath = "/api/v1/query"
		params = map[string]string{"query": q}
	case "query_range":
		q := strArg(in, "query", "q", "expr", "promql")
		if q == "" {
			return "", fmt.Errorf("missing query expression")
		}
		apiPath = "/api/v1/query_range"
		params = map[string]string{
			"query": q,
			"start": strArg(in, "start", "from"),
			"end":   strArg(in, "end", "to"),
			"step":  strArg(in, "step", "interval"),
		}
	case "alerts":
		apiPath = "/api/v1/alerts"
	case "targets":
		apiPath = "/api/v1/targets"
	case "metrics":
		q := strArg(in, "query", "q", "filter")
		if q == "" {
			return "", fmt.Errorf("missing query filter for metrics (e.g., 'up', '{job=\"prometheus\"}', 'node_cpu')")
		}
		apiPath = "/api/v1/series"
		params = map[string]string{"match[]": q}
	}

	u, err := buildPrometheusURL(promURL, apiPath, params)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(u)
	if err != nil {
		return fmt.Sprintf("Failed to connect to Prometheus at %s: %v", promURL, err), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("Prometheus returned HTTP %d: %s", resp.StatusCode, string(body)), nil
	}

	var result struct {
		Status    string `json:"status"`
		Data      any    `json:"data"`
		ErrorType string `json:"errorType"`
		Error     string `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return string(body), nil
	}

	if result.Status != "success" {
		return fmt.Sprintf("Prometheus error (%s): %s", result.ErrorType, result.Error), nil
	}

	pretty, _ := json.MarshalIndent(result.Data, "", "  ")
	return fmt.Sprintf("prometheus %s result:\n\n%s", op, string(pretty)), nil
}

func NewBuiltinPrometheusQueryTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolPrometheusQuery,
			Desc:  "Query Prometheus: PromQL queries, alerts, targets, metrics. Read-only.",
			Extra: map[string]any{"execution_mode": "server"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation":      {Type: einoschema.String, Desc: "Operation: query, query_range, alerts, targets, metrics", Required: false},
				"query":          {Type: einoschema.String, Desc: "PromQL expression (required for query/query_range/metrics ops)", Required: false},
				"prometheus_url": {Type: einoschema.String, Desc: "Prometheus base URL only, e.g. https://prometheus.example.com (default: http://localhost:9090)", Required: false},
				"start":          {Type: einoschema.String, Desc: "Start time for range query", Required: false},
				"end":            {Type: einoschema.String, Desc: "End time for range query", Required: false},
				"step":           {Type: einoschema.String, Desc: "Step interval for range query", Required: false},
			}),
		},
		execBuiltinPrometheusQuery,
	)
}
