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

const toolCronManager = "builtin_cron_manager"

const (
	maxCronWriteBytes = 256 * 1024
	maxCronLineBytes  = 4096
)

var allowedCronOps = map[string]bool{
	"list":        true,
	"system":      true,
	"status":      true,
	"write":       true,
	"append_line": true,
	"clear":       true,
}

func execBuiltinCronManager(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		op = "list"
	}

	if !allowedCronOps[op] {
		return "", fmt.Errorf("operation %q not allowed (allowed: list, system, status, write, append_line, clear)", op)
	}

	switch op {
	case "write":
		return cronWrite(in)
	case "append_line":
		return cronAppendLine(in)
	case "clear":
		return cronClear()
	case "list":
		return cronReadList()
	case "system":
		return cronReadSystem()
	case "status":
		return cronReadStatus()
	default:
		return "", fmt.Errorf("unhandled operation: %s", op)
	}
}

func cronWrite(in map[string]any) (string, error) {
	content := strArg(in, "content", "crontab", "body")
	if strings.Contains(content, "\x00") {
		return "", fmt.Errorf("invalid content")
	}
	if len(content) > maxCronWriteBytes {
		return "", fmt.Errorf("content exceeds max size (%d bytes)", maxCronWriteBytes)
	}
	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(content)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("crontab write failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return fmt.Sprintf("cron write: OK (installed user crontab, %d bytes)", len(content)), nil
}

func cronAppendLine(in map[string]any) (string, error) {
	line := strings.TrimSpace(strArg(in, "line", "entry"))
	if line == "" {
		return "", fmt.Errorf("append_line requires non-empty line")
	}
	if len(line) > maxCronLineBytes {
		return "", fmt.Errorf("line exceeds max length (%d)", maxCronLineBytes)
	}
	if strings.ContainsAny(line, "\n\r") {
		return "", fmt.Errorf("append_line must be a single line")
	}
	existing := ""
	listOut, err := exec.Command("crontab", "-l").CombinedOutput()
	if err == nil {
		existing = strings.TrimSpace(string(listOut))
	}
	var b strings.Builder
	if existing != "" {
		b.WriteString(existing)
		b.WriteByte('\n')
	}
	b.WriteString(line)
	b.WriteByte('\n')
	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(b.String())
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("append_line failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return fmt.Sprintf("cron append_line: OK\nappended: %s", line), nil
}

func cronClear() (string, error) {
	out, err := exec.Command("crontab", "-r").CombinedOutput()
	s := strings.TrimSpace(string(out))
	if err != nil {
		// No crontab is common; still report stderr
		if strings.Contains(s, "no crontab") || strings.Contains(strings.ToLower(s), "no crontab") {
			return "cron clear: no user crontab to remove", nil
		}
		return fmt.Sprintf("cron clear: %v\n%s", err, s), nil
	}
	return "cron clear: OK (user crontab removed)", nil
}

func cronReadList() (string, error) {
	cmd := exec.Command("crontab", "-l")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("cron list: (no entries or permission denied)\n%s", strings.TrimSpace(string(output))), nil
	}
	result := strings.TrimSpace(string(output))
	if result == "" {
		return "cron list: (no entries)", nil
	}
	return fmt.Sprintf("cron list result:\n\n%s", result), nil
}

func cronReadSystem() (string, error) {
	cmd := exec.Command("cat", "/etc/crontab")
	output, err := cmd.CombinedOutput()
	if err != nil {
		cmd = exec.Command("cat", "/etc/cron.d/*")
		output, err = cmd.CombinedOutput()
		if err != nil {
			return "cron system: no entries or permission denied", nil
		}
	}
	result := strings.TrimSpace(string(output))
	if result == "" {
		return "cron system: (no entries)", nil
	}
	return fmt.Sprintf("cron system result:\n\n%s", result), nil
}

func cronReadStatus() (string, error) {
	cmd := exec.Command("launchctl", "list")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("cron status: %v\n%s", err, strings.TrimSpace(string(output))), nil
	}
	result := strings.TrimSpace(string(output))
	if result == "" {
		return "cron status: (no entries)", nil
	}
	return fmt.Sprintf("cron status result:\n\n%s", result), nil
}

func NewBuiltinCronManagerTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name: toolCronManager,
			Desc: "Cron: list/append/write/clear current user's crontab; read system crontab; launchctl list (macOS). " +
				"Write operations affect the current OS user only. Use append_line or write with care.",
			Extra: map[string]any{"execution_mode": "client"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation": {
					Type: einoschema.String,
					Desc: "list | system | status | write | append_line | clear",
					Required: false,
				},
				"content": {
					Type:     einoschema.String,
					Desc:     "Full crontab text for operation=write (replaces user crontab)",
					Required: false,
				},
				"line": {
					Type:     einoschema.String,
					Desc:     "Single cron line for operation=append_line",
					Required: false,
				},
			}),
		},
		execBuiltinCronManager,
	)
}
