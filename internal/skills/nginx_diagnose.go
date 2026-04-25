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

const toolNginxDiagnose = "builtin_nginx_diagnose"

var allowedNginxOps = map[string]bool{
	"test_config": true,
	"show_config": true,
	"list_sites":  true,
	"status":      true,
}

func execBuiltinNginxDiagnose(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		op = "test_config"
	}

	if !allowedNginxOps[op] {
		return "", fmt.Errorf("operation %q not allowed (read-only mode; allowed: %v)", op, allowedNginxOps)
	}

	var cmd *exec.Cmd
	switch op {
	case "test_config":
		cmd = exec.Command("nginx", "-t")
	case "show_config":
		cmd = exec.Command("nginx", "-T")
	case "list_sites":
		cmd = exec.Command("ls", "/etc/nginx/sites-enabled")
	case "status":
		cmd = exec.Command("ps", "aux")
	}

	output, err := cmd.CombinedOutput()
	result := strings.TrimSpace(string(output))

	if op == "status" && err == nil {
		lines := strings.Split(result, "\n")
		var nginxLines []string
		for _, line := range lines {
			if strings.Contains(line, "nginx") {
				nginxLines = append(nginxLines, line)
			}
		}
		if len(nginxLines) == 0 {
			return "nginx: not running", nil
		}
		result = strings.Join(nginxLines, "\n")
	}

	if err != nil && op != "list_sites" {
		return fmt.Sprintf("nginx %s: %v\n%s", op, err, result), nil
	}

	if result == "" {
		return fmt.Sprintf("nginx %s: (no output)", op), nil
	}

	return fmt.Sprintf("nginx %s result:\n\n%s", op, result), nil
}

func NewBuiltinNginxDiagnoseTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolNginxDiagnose,
			Desc:  "Analyze Nginx: test config syntax, show active config, list sites, check status. Read-only.",
			Extra: map[string]any{"execution_mode": "client"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation": {Type: einoschema.String, Desc: "Operation: test_config, show_config, list_sites, status", Required: false},
			}),
		},
		execBuiltinNginxDiagnose,
	)
}
