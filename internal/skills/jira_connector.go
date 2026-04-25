package skills

import (
	"context"
	"encoding/base64"
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

const toolJiraConnector = "builtin_jira_connector"

var allowedJiraOps = map[string]bool{
	"search":   true,
	"issue":    true,
	"projects": true,
	"board":    true,
}

func execBuiltinJiraConnector(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		op = "search"
	}

	if !allowedJiraOps[op] {
		return "", fmt.Errorf("operation %q not allowed (read-only mode; allowed: %v)", op, allowedJiraOps)
	}

	jiraURL := strArg(in, "jira_url", "url", "endpoint")
	if jiraURL == "" {
		return "", fmt.Errorf("missing jira_url")
	}
	jiraURL = strings.TrimRight(jiraURL, "/")

	apiToken := strArg(in, "api_token", "token", "password")
	email := strArg(in, "email", "user", "username")

	var apiPath string
	var params map[string]string

	switch op {
	case "search":
		jql := strArg(in, "jql", "query", "q")
		if jql == "" {
			return "", fmt.Errorf("missing jql query")
		}
		apiPath = "/rest/api/3/search"
		params = map[string]string{"jql": jql, "maxResults": "20"}
	case "issue":
		key := strArg(in, "issue_key", "key", "id")
		if key == "" {
			return "", fmt.Errorf("missing issue_key")
		}
		apiPath = "/rest/api/3/issue/" + key
	case "projects":
		apiPath = "/rest/api/3/project"
	case "board":
		apiPath = "/rest/agile/1.0/board"
	}

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", jiraURL+apiPath, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if apiToken != "" && email != "" {
		auth := base64.StdEncoding.EncodeToString([]byte(email + ":" + apiToken))
		req.Header.Set("Authorization", "Basic "+auth)
	} else if apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+apiToken)
	}

	if len(params) > 0 {
		qs := req.URL.Query()
		for k, v := range params {
			qs.Set(k, v)
		}
		req.URL.RawQuery = qs.Encode()
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("Failed to connect to Jira at %s: %v", jiraURL, err), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("Jira returned HTTP %d: %s", resp.StatusCode, string(body)), nil
	}

	var result any
	if err := json.Unmarshal(body, &result); err != nil {
		return string(body), nil
	}

	pretty, _ := json.MarshalIndent(result, "", "  ")
	return fmt.Sprintf("jira %s result:\n\n%s", op, string(pretty)), nil
}

func NewBuiltinJiraConnectorTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolJiraConnector,
			Desc:  "Query Jira: search issues with JQL, get issue details, list projects, boards. Read-only.",
			Extra: map[string]any{"execution_mode": "server"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation": {Type: einoschema.String, Desc: "Operation: search, issue, projects, board", Required: false},
				"jql":       {Type: einoschema.String, Desc: "JQL query (for search)", Required: false},
				"issue_key": {Type: einoschema.String, Desc: "Issue key (e.g., PROJ-123)", Required: false},
				"jira_url":  {Type: einoschema.String, Desc: "Jira server URL", Required: false},
				"api_token": {Type: einoschema.String, Desc: "Jira API token", Required: false},
				"email":     {Type: einoschema.String, Desc: "Jira email/username", Required: false},
			}),
		},
		execBuiltinJiraConnector,
	)
}
