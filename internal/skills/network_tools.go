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

const toolNetworkTools = "builtin_network_tools"

var allowedNetworkOps = map[string]bool{
	"ping":        true,
	"traceroute":  true,
	"connections": true,
	"listening":   true,
}

func execBuiltinNetworkTools(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		op = "connections"
	}

	if !allowedNetworkOps[op] {
		return "", fmt.Errorf("operation %q not allowed (read-only mode; allowed: %v)", op, allowedNetworkOps)
	}

	var cmd *exec.Cmd
	switch op {
	case "ping":
		host := strArg(in, "host", "target", "address")
		if host == "" {
			return "", fmt.Errorf("missing host for ping")
		}
		count := strArg(in, "count", "n", "times")
		if count == "" {
			count = "4"
		}
		cmd = exec.Command("ping", "-c", count, host)
	case "traceroute":
		host := strArg(in, "host", "target", "address")
		if host == "" {
			return "", fmt.Errorf("missing host for traceroute")
		}
		cmd = exec.Command("traceroute", host)
	case "connections":
		cmd = exec.Command("netstat", "-an")
	case "listening":
		cmd = exec.Command("lsof", "-i", "-P", "-n")
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("network %s: error or no results (%v)", op, err), nil
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		return fmt.Sprintf("network %s: (no output)", op), nil
	}

	return fmt.Sprintf("network %s result:\n\n%s", op, result), nil
}

func NewBuiltinNetworkToolsTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolNetworkTools,
			Desc:  "Network diagnostics: ping, traceroute, connections, listening ports. Read-only.",
			Extra: map[string]any{"execution_mode": "client"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation": {Type: einoschema.String, Desc: "Operation: ping, traceroute, connections, listening", Required: false},
				"host":      {Type: einoschema.String, Desc: "Target host for ping/traceroute", Required: false},
				"count":     {Type: einoschema.String, Desc: "Ping count (default: 4)", Required: false},
			}),
		},
		execBuiltinNetworkTools,
	)
}
