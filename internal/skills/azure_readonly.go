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

const toolAzureReadonly = "builtin_azure_readonly"

var allowedAzureOps = map[string]bool{
	"vm":      true,
	"groups":  true,
	"storage": true,
	"aks":     true,
}

func execBuiltinAzureReadonly(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		op = "vm"
	}
	if !allowedAzureOps[op] {
		return "", fmt.Errorf("operation %q not allowed (read-only; allowed: %v)", op, allowedAzureOps)
	}

	sub := strArg(in, "subscription", "subscription_id")

	var cmd *exec.Cmd
	switch op {
	case "vm":
		args := []string{"vm", "list", "-o", "table"}
		if sub != "" {
			args = append([]string{"--subscription", sub}, args...)
		}
		cmd = exec.Command("az", args...)
	case "groups":
		args := []string{"group", "list", "-o", "table"}
		if sub != "" {
			args = append([]string{"--subscription", sub}, args...)
		}
		cmd = exec.Command("az", args...)
	case "storage":
		args := []string{"storage", "account", "list", "-o", "table"}
		if sub != "" {
			args = append([]string{"--subscription", sub}, args...)
		}
		cmd = exec.Command("az", args...)
	case "aks":
		args := []string{"aks", "list", "-o", "table"}
		if sub != "" {
			args = append([]string{"--subscription", sub}, args...)
		}
		cmd = exec.Command("az", args...)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("Azure %s: %v\n%s", op, err, string(output)), nil
	}
	result := strings.TrimSpace(string(output))
	if result == "" {
		return fmt.Sprintf("Azure %s: no results", op), nil
	}
	return fmt.Sprintf("Azure %s:\n\n%s", op, result), nil
}

func NewBuiltinAzureReadonlyTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolAzureReadonly,
			Desc:  "Read-only Azure via az CLI: VMs, resource groups, storage accounts, AKS. Requires Azure CLI.",
			Extra: map[string]any{"execution_mode": "server"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation":    {Type: einoschema.String, Desc: "Operation: vm, groups, storage, aks", Required: false},
				"subscription": {Type: einoschema.String, Desc: "Optional subscription id or name", Required: false},
			}),
		},
		execBuiltinAzureReadonly,
	)
}
