package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
)

const toolLokiQuery = "builtin_loki_query"

const lokiMaxLimit = 5000

var allowedLokiOps = map[string]bool{
	"query":        true,
	"query_range":  true,
	"labels":       true,
	"label_values": true,
	"series":       true,
}

func execBuiltinLokiQuery(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		op = "query_range"
	}
	if !allowedLokiOps[op] {
		return "", fmt.Errorf("operation %q not allowed (read-only; allowed: %v)", op, allowedLokiOps)
	}

	base := strings.TrimSpace(strArg(in, "loki_url", "url", "endpoint"))
	if base == "" {
		base = "http://localhost:3100"
	}
	base = strings.TrimRight(base, "/")

	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("invalid loki_url: need http(s) URL with host")
	}

	q := u.Query()
	var apiPath string

	switch op {
	case "query":
		logql := strArg(in, "query", "q", "logql")
		if logql == "" {
			return "", fmt.Errorf("missing LogQL query expression")
		}
		apiPath = "/loki/api/v1/query"
		q.Set("query", logql)
		if t := strArg(in, "time", "at"); t != "" {
			q.Set("time", t)
		}
	case "query_range":
		logql := strArg(in, "query", "q", "logql")
		if logql == "" {
			return "", fmt.Errorf("missing LogQL query expression")
		}
		apiPath = "/loki/api/v1/query_range"
		q.Set("query", logql)
		if s := strArg(in, "start", "from"); s != "" {
			q.Set("start", normalizeLokiTime(s))
		}
		if e := strArg(in, "end", "to"); e != "" {
			q.Set("end", normalizeLokiTime(e))
		}
		if st := strArg(in, "step"); st != "" {
			q.Set("step", st)
		}
		lim := strArg(in, "limit", "max_lines")
		if lim == "" {
			lim = "500"
		}
		if n, err := strconv.Atoi(lim); err == nil && n > lokiMaxLimit {
			lim = strconv.Itoa(lokiMaxLimit)
		}
		q.Set("limit", lim)
	case "labels":
		apiPath = "/loki/api/v1/labels"
		if s := strArg(in, "start", "from"); s != "" {
			q.Set("start", normalizeLokiTime(s))
		}
		if e := strArg(in, "end", "to"); e != "" {
			q.Set("end", normalizeLokiTime(e))
		}
	case "label_values":
		label := strArg(in, "label", "name")
		if label == "" {
			return "", fmt.Errorf("missing label name for label_values")
		}
		apiPath = "/loki/api/v1/label/" + url.PathEscape(label) + "/values"
		if s := strArg(in, "start", "from"); s != "" {
			q.Set("start", normalizeLokiTime(s))
		}
		if e := strArg(in, "end", "to"); e != "" {
			q.Set("end", normalizeLokiTime(e))
		}
	case "series":
		apiPath = "/loki/api/v1/series"
		m := strArg(in, "match", "selector", "query")
		if m == "" {
			return "", fmt.Errorf("missing match selector for series (e.g. '{job=\"varlogs\"}')")
		}
		q.Add("match[]", m)
		if s := strArg(in, "start", "from"); s != "" {
			q.Set("start", normalizeLokiTime(s))
		}
		if e := strArg(in, "end", "to"); e != "" {
			q.Set("end", normalizeLokiTime(e))
		}
	}

	u.Path = apiPath
	u.RawQuery = q.Encode()
	finalURL := u.String()

	req, err := http.NewRequest(http.MethodGet, finalURL, nil)
	if err != nil {
		return "", err
	}
	if tok := strings.TrimSpace(strArg(in, "bearer_token", "token")); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("Failed to reach Loki at %s: %v", base, err), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("Loki returned HTTP %d: %s", resp.StatusCode, string(body)), nil
	}

	var result struct {
		Status string `json:"status"`
		Data   any    `json:"data"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return string(body), nil
	}
	if result.Status != "success" {
		return fmt.Sprintf("Loki error: %s", result.Error), nil
	}
	pretty, _ := json.MarshalIndent(result.Data, "", "  ")
	return fmt.Sprintf("loki %s result:\n\n%s", op, string(pretty)), nil
}

// normalizeLokiTime converts to Unix nanoseconds string for Loki query_range parameters.
func normalizeLokiTime(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		if n < 1e12 { // seconds
			return strconv.FormatInt(n*1e9, 10)
		}
		return strconv.FormatInt(n, 10) // already nanoseconds
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return strconv.FormatInt(t.UnixNano(), 10)
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return strconv.FormatInt(t.UnixNano(), 10)
	}
	return s
}

func NewBuiltinLokiQueryTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolLokiQuery,
			Desc:  "Query Grafana Loki: LogQL instant/range, labels, label values, series. Read-only HTTP API.",
			Extra: map[string]any{"execution_mode": "server"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation":   {Type: einoschema.String, Desc: "Operation: query, query_range, labels, label_values, series", Required: false},
				"query":       {Type: einoschema.String, Desc: "LogQL (required for query, query_range, series uses match)", Required: false},
				"loki_url":    {Type: einoschema.String, Desc: "Loki base URL (default: http://localhost:3100)", Required: false},
				"start":       {Type: einoschema.String, Desc: "Range start (RFC3339 or Unix seconds)", Required: false},
				"end":         {Type: einoschema.String, Desc: "Range end (RFC3339 or Unix seconds)", Required: false},
				"step":        {Type: einoschema.String, Desc: "Resolution step for query_range", Required: false},
				"limit":       {Type: einoschema.String, Desc: "Max lines for query_range (capped)", Required: false},
				"label":       {Type: einoschema.String, Desc: "Label name for label_values", Required: false},
				"match":       {Type: einoschema.String, Desc: "Series selector for series op, e.g. {job=\"app\"}", Required: false},
				"bearer_token": {Type: einoschema.String, Desc: "Optional Bearer token for auth", Required: false},
			}),
		},
		execBuiltinLokiQuery,
	)
}
