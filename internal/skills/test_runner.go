package skills

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
)

const toolTestRunner = "builtin_test_runner"

func execBuiltinTestRunner(_ context.Context, in map[string]any) (string, error) {
	framework := strArg(in, "framework", "lang", "language")
	if framework == "" {
		framework = "go"
	}
	framework = strings.ToLower(framework)

	workDir := strArg(in, "work_dir", "cwd", "directory")
	testPath := strArg(in, "path", "test_path", "file")
	pattern := strArg(in, "pattern", "match", "run")

	var cmd *exec.Cmd
	switch framework {
	case "go":
		args := []string{"test", "-v"}
		if testPath != "" {
			args = append(args, testPath)
		}
		if pattern != "" {
			args = append(args, "-run", pattern)
		}
		args = append(args, "-count=1")
		cmd = exec.CommandContext(context.Background(), "go", args...)
	case "jest", "npm":
		if _, err := os.Stat("package.json"); err == nil {
			cmd = exec.CommandContext(context.Background(), "npm", "test", "--", "--verbose")
		} else {
			cmd = exec.CommandContext(context.Background(), "npx", "jest", "--verbose")
		}
	case "pytest":
		args := []string{"-v"}
		if pattern != "" {
			args = append(args, "-k", pattern)
		}
		if testPath != "" {
			args = append(args, testPath)
		} else {
			args = append(args, ".")
		}
		cmd = exec.CommandContext(context.Background(), "pytest", args...)
	case "cargo":
		args := []string{"test", "--", "--verbose"}
		if pattern != "" {
			args = append(args, "--test", pattern)
		}
		cmd = exec.CommandContext(context.Background(), "cargo", args...)
	default:
		return "", fmt.Errorf("unsupported framework: %s", framework)
	}

	if workDir != "" {
		fi, err := os.Stat(workDir)
		if err != nil || !fi.IsDir() {
			return "", fmt.Errorf("invalid work_dir: %w", err)
		}
		cmd.Dir = workDir
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("Test execution failed: %v\n\nOutput:\n%s", err, string(output)), nil
	}

	return fmt.Sprintf("Test executed successfully:\n%s", string(output)), nil
}

func NewBuiltinTestRunnerTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name: toolTestRunner,
			Desc: "Run unit tests for Go, Jest, Pytest, Cargo, and other frameworks.",
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"framework": {Type: einoschema.String, Desc: "Test framework: go, jest, pytest, cargo", Required: false},
				"path":      {Type: einoschema.String, Desc: "Test file or directory path", Required: false},
				"pattern":   {Type: einoschema.String, Desc: "Test name pattern to match", Required: false},
				"work_dir":  {Type: einoschema.String, Desc: "Working directory", Required: false},
				"flags":     {Type: einoschema.String, Desc: "Additional flags as JSON array", Required: false},
			}),
		},
		execBuiltinTestRunner,
	)
}
