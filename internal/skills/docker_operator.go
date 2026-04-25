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

const toolDockerOperator = "builtin_docker_operator"

var allowedDockerOps = map[string]bool{
	"ps":      true,
	"images":  true,
	"logs":    true,
	"inspect": true,
	"stats":   true,
	"network": true,
	"volume":  true,
	"info":    true,
	"version": true,
	"events":  true,
}

func execBuiltinDockerOperator(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action", "command")
	if op == "" {
		op = "ps"
	}

	if !allowedDockerOps[op] {
		return "", fmt.Errorf("operation %q not allowed (read-only mode; allowed: %v)", op, allowedDockerOps)
	}

	name := strArg(in, "name", "container", "target")
	allFlag := strArg(in, "all", "include_stopped", "show_all")

	cmdArgs := []string{op}

	switch op {
	case "ps":
		if allFlag == "true" || allFlag == "1" || allFlag == "yes" {
			cmdArgs = append(cmdArgs, "-a")
		}
	case "logs":
		if name == "" {
			return "", fmt.Errorf("missing container name for logs")
		}
		cmdArgs = append(cmdArgs, "--tail", "100", name)
	case "inspect":
		if name == "" {
			return "", fmt.Errorf("missing container/image name for inspect")
		}
		cmdArgs = append(cmdArgs, name)
	case "stats":
		if name != "" {
			cmdArgs = append(cmdArgs, "--no-stream", name)
		} else {
			cmdArgs = append(cmdArgs, "--no-stream")
		}
	case "network":
		subOp := strArg(in, "sub_operation", "sub_op", "action2")
		if subOp == "" {
			subOp = "ls"
		}
		cmdArgs = append(cmdArgs, subOp)
		if name != "" && subOp == "inspect" {
			cmdArgs = append(cmdArgs, name)
		}
	case "volume":
		subOp := strArg(in, "sub_operation", "sub_op", "action2")
		if subOp == "" {
			subOp = "ls"
		}
		cmdArgs = append(cmdArgs, subOp)
		if name != "" && subOp == "inspect" {
			cmdArgs = append(cmdArgs, name)
		}
	case "images":
		cmdArgs = append(cmdArgs, "-a")
	}

	cmd := exec.Command("docker", cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker %s failed: %s\n%s", op, err.Error(), string(output))
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		return fmt.Sprintf("docker %s: (no output)", op), nil
	}

	return fmt.Sprintf("docker %s result:\n\n%s", op, result), nil
}

func NewBuiltinDockerOperatorTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolDockerOperator,
			Desc:  "Read-only Docker operations: ps, images, logs, inspect, stats, network, volume. Write operations are disabled for safety.",
			Extra: map[string]any{"execution_mode": "client"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation":     {Type: einoschema.String, Desc: "Operation: ps, images, logs, inspect, stats, network, volume", Required: false},
				"name":          {Type: einoschema.String, Desc: "Container/image/network/volume name", Required: false},
				"all":           {Type: einoschema.String, Desc: "Include stopped containers (true/false)", Required: false},
				"sub_operation": {Type: einoschema.String, Desc: "Sub-operation for network/volume (ls, inspect)", Required: false},
			}),
		},
		execBuiltinDockerOperator,
	)
}
