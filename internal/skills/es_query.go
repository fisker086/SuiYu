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

const toolESQuery = "builtin_es_query"

var allowedESOps = map[string]bool{
	"search":         true,
	"count":          true,
	"mapping":        true,
	"indices":        true,
	"cluster_health": true,
	"aliases":        true,
}

func execBuiltinESQuery(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		op = "search"
	}

	if !allowedESOps[op] {
		return "", fmt.Errorf("operation %q not allowed (allowed: %v)", op, allowedESOps)
	}

	addresses := strArg(in, "addresses", "hosts", "es_hosts")
	if addresses == "" {
		return "", fmt.Errorf("missing ES addresses (comma-separated)")
	}

	username := strArg(in, "username", "user")
	password := strArg(in, "password", "pass")
	apiKey := strArg(in, "api_key", "apikey")

	index := strArg(in, "index", "indices", "target")
	if index == "" && op != "indices" && op != "cluster_health" && op != "aliases" {
		return "", fmt.Errorf("missing index")
	}

	timeoutSec := strArg(in, "timeout", "timeout_seconds")
	timeout := 30 * time.Second
	if timeoutSec != "" {
		var t int
		if _, err := fmt.Sscanf(timeoutSec, "%d", &t); err == nil && t > 0 {
			timeout = time.Duration(t) * time.Second
		}
	}

	hosts := strings.Split(addresses, ",")
	for i := range hosts {
		hosts[i] = strings.TrimSpace(hosts[i])
		if !strings.HasPrefix(hosts[i], "http") {
			hosts[i] = "http://" + hosts[i]
		}
	}

	client := &http.Client{Timeout: timeout}

	var path string
	var body io.Reader

	switch op {
	case "search":
		query := strArg(in, "query", "body", "es_query")
		if query == "" {
			query = `{"query": {"match_all": {}}}`
		}
		size := strArg(in, "size", "limit")
		if size == "" {
			size = "10"
		}
		source := strArg(in, "_source", "source", "fields")
		path = fmt.Sprintf("/%s/_search?size=%s", index, size)
		if source != "" {
			path += "&_source=" + source
		}
		body = strings.NewReader(query)

	case "count":
		query := strArg(in, "query", "body", "es_query")
		if query == "" {
			query = `{"query": {"match_all": {}}}`
		}
		path = fmt.Sprintf("/%s/_count", index)
		body = strings.NewReader(query)

	case "mapping":
		path = fmt.Sprintf("/%s/_mapping", index)

	case "indices":
		path = "/_cat/indices?v"

	case "cluster_health":
		path = "/_cluster/health?pretty"

	case "aliases":
		path = "/_cat/aliases?v"
	}

	req, err := http.NewRequest("POST", hosts[0]+path, body)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	if apiKey != "" {
		req.Header.Set("Authorization", "ApiKey "+apiKey)
	} else if username != "" && password != "" {
		req.SetBasicAuth(username, password)
	}

	req.Header.Set("Content-Type", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Sprintf("ES returned HTTP %d:\n%s", resp.StatusCode, string(respBody)), nil
	}

	var result any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return string(respBody), nil
	}

	pretty, _ := json.MarshalIndent(result, "", "  ")

	switch op {
	case "search":
		return fmt.Sprintf("ES search on '%s':\n\n%s", index, string(pretty)), nil
	case "count":
		return fmt.Sprintf("ES count on '%s':\n\n%s", index, string(pretty)), nil
	case "mapping":
		return fmt.Sprintf("ES mapping for '%s':\n\n%s", index, string(pretty)), nil
	case "indices":
		return "ES indices:\n\n" + string(pretty), nil
	case "cluster_health":
		return "ES cluster health:\n\n" + string(pretty), nil
	case "aliases":
		return "ES aliases:\n\n" + string(pretty), nil
	default:
		return string(pretty), nil
	}
}

func NewBuiltinESQueryTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name: toolESQuery,
			Desc: "Elasticsearch operations: search, count, mapping, indices, cluster health, aliases. Supports basic auth and API key.",
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation": {Type: einoschema.String, Desc: "Operation: search, count, mapping, indices, cluster_health, aliases", Required: false},
				"addresses": {Type: einoschema.String, Desc: "ES addresses (comma-separated, e.g., localhost:9200)", Required: true},
				"index":     {Type: einoschema.String, Desc: "Index name to query", Required: false},
				"username":  {Type: einoschema.String, Desc: "ES username (optional)", Required: false},
				"password":  {Type: einoschema.String, Desc: "ES password (optional)", Required: false},
				"api_key":   {Type: einoschema.String, Desc: "ES API key (optional, alternative to username/password)", Required: false},
				"query":     {Type: einoschema.String, Desc: "ES query JSON (for search/count)", Required: false},
				"size":      {Type: einoschema.String, Desc: "Number of results (default: 10)", Required: false},
				"_source":   {Type: einoschema.String, Desc: "Fields to return (comma-separated)", Required: false},
				"timeout":   {Type: einoschema.String, Desc: "Request timeout in seconds (default: 30)", Required: false},
			}),
		},
		execBuiltinESQuery,
	)
}
