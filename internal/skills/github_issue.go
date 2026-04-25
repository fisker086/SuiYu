package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
)

const toolGitHubIssue = "builtin_github_issue"

var allowedGHOps = map[string]bool{
	"issues": true,
	"issue":  true,
	"prs":    true,
	"repo":   true,
}

func execBuiltinGitHubIssue(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		op = "issues"
	}

	if !allowedGHOps[op] {
		return "", fmt.Errorf("operation %q not allowed (read-only mode; allowed: %v)", op, allowedGHOps)
	}

	repo := strArg(in, "repo", "repository", "owner_repo")
	if repo == "" {
		return "", fmt.Errorf("missing repo (format: owner/repo)")
	}

	token := strArg(in, "token", "github_token", "api_token")

	var apiPath string
	switch op {
	case "issues":
		state := strArg(in, "state", "filter")
		if state == "" {
			state = "open"
		}
		apiPath = fmt.Sprintf("/repos/%s/issues?state=%s&per_page=20", repo, state)
	case "issue":
		num := strArg(in, "issue_number", "number", "id")
		if num == "" {
			return "", fmt.Errorf("missing issue_number")
		}
		apiPath = fmt.Sprintf("/repos/%s/issues/%s", repo, num)
	case "prs":
		state := strArg(in, "state", "filter")
		if state == "" {
			state = "open"
		}
		apiPath = fmt.Sprintf("/repos/%s/pulls?state=%s&per_page=20", repo, state)
	case "repo":
		apiPath = fmt.Sprintf("/repos/%s", repo)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", "https://api.github.com"+apiPath, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("Failed to connect to GitHub API: %v", err), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("GitHub returned HTTP %d: %s", resp.StatusCode, string(body)), nil
	}

	var result any
	if err := json.Unmarshal(body, &result); err != nil {
		return string(body), nil
	}

	pretty, _ := json.MarshalIndent(result, "", "  ")
	return fmt.Sprintf("github %s (%s):\n\n%s", op, repo, string(pretty)), nil
}

func NewBuiltinGitHubIssueTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolGitHubIssue,
			Desc:  "Query GitHub: list issues, get issue details, list PRs, repo info. Read-only.",
			Extra: map[string]any{"execution_mode": "server"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation":    {Type: einoschema.String, Desc: "Operation: issues, issue, prs, repo", Required: false},
				"repo":         {Type: einoschema.String, Desc: "Repository (owner/repo)", Required: true},
				"issue_number": {Type: einoschema.String, Desc: "Issue/PR number", Required: false},
				"state":        {Type: einoschema.String, Desc: "Filter: open, closed, all", Required: false},
				"token":        {Type: einoschema.String, Desc: "GitHub token (optional)", Required: false},
			}),
		},
		execBuiltinGitHubIssue,
	)
}
