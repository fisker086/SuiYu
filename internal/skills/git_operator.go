package skills

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
)

const toolGitOperator = "builtin_git_operator"

var allowedGitOps = map[string]bool{
	"status": true,
	"log":    true,
	"diff":   true,
	"branch": true,
	"show":   true,
	"blame":  true,
	"tag":    true,
}

func execBuiltinGitOperator(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		op = "status"
	}

	if !allowedGitOps[op] {
		return "", fmt.Errorf("operation %q not allowed (read-only mode; allowed: %v)", op, allowedGitOps)
	}

	repoPath := strArg(in, "repo_path", "path", "directory", "dir")
	if repoPath == "" {
		repoPath = "."
	}

	args := strArg(in, "args", "arguments", "extra")

	cmdArgs := buildGitArgs(op, args)
	cmd := exec.Command("git", cmdArgs...)
	cmd.Dir = repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s failed in %s: %s\n%s", op, repoPath, err.Error(), string(output))
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		return fmt.Sprintf("git %s: (no output)", op), nil
	}

	return fmt.Sprintf("git %s in %s:\n\n%s", op, repoPath, result), nil
}

func buildGitArgs(op string, extraArgs string) []string {
	base := []string{op}

	if extraArgs != "" {
		parts := strings.Fields(extraArgs)
		base = append(base, parts...)
	}

	switch op {
	case "log":
		if extraArgs == "" {
			base = append(base, "--oneline", "-n", "20")
		}
	case "diff":
		if extraArgs == "" {
			base = append(base, "--stat")
		}
	case "branch":
		if extraArgs == "" {
			base = append(base, "-a")
		}
	}

	return base
}

func NewBuiltinGitOperatorTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolGitOperator,
			Desc:  "Read-only git operations: status, log, diff, branch, show, blame, tag. Write operations (commit, push, merge) are disabled for safety.",
			Extra: map[string]any{"execution_mode": "client"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation": {Type: einoschema.String, Desc: "Operation: status, log, diff, branch, show, blame, tag", Required: false},
				"repo_path": {Type: einoschema.String, Desc: "Path to git repository (default: current directory)", Required: false},
				"args":      {Type: einoschema.String, Desc: "Additional git arguments", Required: false},
			}),
		},
		execBuiltinGitOperator,
	)
}
