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

const toolSystemMonitor = "builtin_system_monitor"

var allowedSystemMonitorOps = map[string]bool{
	"cpu":       true,
	"memory":    true,
	"disk":      true,
	"processes": true,
	"uptime":    true,
	"all":       true,
}

func execBuiltinSystemMonitor(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		op = "all"
	}

	if !allowedSystemMonitorOps[op] {
		return "", fmt.Errorf("operation %q not allowed (read-only mode; allowed: %v)", op, allowedSystemMonitorOps)
	}

	var results []string

	runOps := func(ops []string) error {
		for _, o := range ops {
			var cmd *exec.Cmd
			var label string
			switch o {
			case "cpu":
				cmd = exec.Command("top", "-l", "1", "-s", "0")
				label = "CPU"
			case "memory":
				cmd = exec.Command("vm_stat")
				label = "Memory"
			case "disk":
				cmd = exec.Command("df", "-h")
				label = "Disk"
			case "processes":
				limit := strArg(in, "limit", "count", "n")
				if limit == "" {
					limit = "10"
				}
				cmd = exec.Command("ps", "aux", "-r")
				label = "Processes"
			case "uptime":
				cmd = exec.Command("uptime")
				label = "Uptime"
			}

			if cmd == nil {
				continue
			}

			output, err := cmd.CombinedOutput()
			if err != nil {
				results = append(results, fmt.Sprintf("=== %s ===\nError: %v", label, err))
				continue
			}

			out := strings.TrimSpace(string(output))
			if out == "" {
				out = "(no output)"
			}
			results = append(results, fmt.Sprintf("=== %s ===\n%s", label, out))
		}
		return nil
	}

	if op == "all" {
		if err := runOps([]string{"cpu", "memory", "disk", "uptime"}); err != nil {
			return "", err
		}
	} else {
		if err := runOps([]string{op}); err != nil {
			return "", err
		}
	}

	return strings.Join(results, "\n\n"), nil
}

func NewBuiltinSystemMonitorTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolSystemMonitor,
			Desc:  "Monitor system resources: CPU, memory, disk, processes, uptime. Read-only.",
			Extra: map[string]any{"execution_mode": "client"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation": {Type: einoschema.String, Desc: "Operation: cpu, memory, disk, processes, uptime, all", Required: false},
				"limit":     {Type: einoschema.String, Desc: "Number of top processes (default: 10)", Required: false},
			}),
		},
		execBuiltinSystemMonitor,
	)
}
