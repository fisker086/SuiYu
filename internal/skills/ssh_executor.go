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

const toolSSHExecutor = "builtin_ssh_executor"

var blockedSSHCommands = []string{
	"rm ", "mv ", "cp ", "chmod ", "chown ", "chgrp ",
	"dd ", "mkfs", "fdisk", "parted",
	"iptables ", "firewall-cmd",
	"kill ", "killall", "pkill",
	"shutdown", "reboot", "halt", "poweroff",
	"passwd", "useradd", "userdel", "usermod", "groupadd", "groupdel",
	"sudo ", "su ",
	"wget ", "curl ", "scp ", "rsync ",
}

func execBuiltinSSHExecutor(_ context.Context, in map[string]any) (string, error) {
	host := strArg(in, "host", "server", "hostname", "address")
	if host == "" {
		return "", fmt.Errorf("missing host")
	}

	command := strArg(in, "command", "cmd", "exec")
	if command == "" {
		return "", fmt.Errorf("missing command")
	}

	if !isSafeSSHCommand(command) {
		return "", fmt.Errorf("command contains blocked operations for safety (read-only mode)")
	}

	port := strArg(in, "port", "ssh_port")
	if port == "" {
		port = "22"
	}

	user := strArg(in, "user", "username", "login")
	if user == "" {
		user = "root"
	}

	cmd := exec.Command("ssh", "-o", "StrictHostKeyChecking=no", "-o", "ConnectTimeout=10", "-p", port, fmt.Sprintf("%s@%s", user, host), command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("SSH command failed on %s: %s\n%s", host, err.Error(), string(output))
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		return fmt.Sprintf("SSH command executed on %s (no output)", host), nil
	}

	return fmt.Sprintf("SSH result from %s@%s:%s:\n\n%s", user, host, port, result), nil
}

func isSafeSSHCommand(command string) bool {
	lower := strings.ToLower(strings.TrimSpace(command))

	allowedPrefixes := []string{
		"cat ", "ls ", "df ", "du ", "free ", "top ", "ps ",
		"uptime", "uname", "hostname", "whoami", "id",
		"tail ", "head ", "grep ", "wc ", "sort ", "uniq ",
		"find ", "stat ", "file ", "which ", "whereis ",
		"netstat ", "ss ", "ip ", "ifconfig ", "ping ", "traceroute ",
		"systemctl status", "journalctl", "dmesg",
		"vmstat", "iostat", "mpstat", "sar",
		"crontab -l", "env", "echo ", "date", "who ", "w ", "last ",
		"docker ps", "docker images", "docker logs", "docker inspect", "docker stats",
		"kubectl get", "kubectl describe", "kubectl logs", "kubectl top", "kubectl events",
	}

	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(lower, prefix) || lower == strings.TrimSuffix(prefix, " ") {
			return true
		}
	}

	for _, blocked := range blockedSSHCommands {
		if strings.Contains(lower, blocked) {
			return false
		}
	}

	if strings.Contains(lower, ">") || strings.Contains(lower, ">>") || strings.Contains(lower, "|") {
		return false
	}

	return true
}

func NewBuiltinSSHExecutorTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name: toolSSHExecutor,
			Desc: "Read-only SSH command execution for remote server inspection. Write operations are blocked for safety.",
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"host":    {Type: einoschema.String, Desc: "Remote server hostname or IP", Required: true},
				"port":    {Type: einoschema.String, Desc: "SSH port (default: 22)", Required: false},
				"user":    {Type: einoschema.String, Desc: "SSH username (default: root)", Required: false},
				"command": {Type: einoschema.String, Desc: "Command to execute (read-only commands only)", Required: true},
			}),
		},
		execBuiltinSSHExecutor,
	)
}
