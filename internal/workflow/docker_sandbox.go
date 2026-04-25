package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/fisk086/sya/internal/logger"
)

type DockerResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Error    error
}

func (s *DockerSandbox) Execute(ctx context.Context, language, code string, input map[string]any) (string, error) {
	result := s.executeWithDocker(ctx, language, code, input)

	if result.Error != nil {
		return "", result.Error
	}

	if result.Stderr != "" && result.ExitCode != 0 {
		return result.Stdout, fmt.Errorf(result.Stderr)
	}

	return result.Stdout, nil
}

func (s *DockerSandbox) executeWithDocker(ctx context.Context, language, code string, input map[string]any) *DockerResult {
	if err := checkDocker(); err != nil {
		logger.Warn("docker sandbox: check failed before run", "language", language, "err", err.Error())
		return &DockerResult{Error: fmt.Errorf("docker not available: %v", err)}
	}

	imageName := s.getImage(language)
	containerName := fmt.Sprintf("taskmate-sandbox-%d", time.Now().UnixNano())
	logger.Info("docker sandbox: starting", "language", language, "image", imageName, "container", containerName)

	defer func() {
		_ = exec.Command("docker", "rm", "-f", containerName).Run()
		logger.Debug("docker sandbox: container removed", "container", containerName)
	}()

	inputJSON := mapToJSON(input)
	runnerCode := generateRunnerCode(language, code, inputJSON)

	tempDir, err := os.MkdirTemp("", "taskmate-sandbox-")
	if err != nil {
		return &DockerResult{Error: fmt.Errorf("failed to create temp dir: %w", err)}
	}
	defer os.RemoveAll(tempDir)

	codeFile := filepath.Join(tempDir, "script."+getFileExtension(language))
	if err := os.WriteFile(codeFile, []byte(runnerCode), 0644); err != nil {
		return &DockerResult{Error: fmt.Errorf("failed to write code file: %w", err)}
	}

	go func() {
		if out, err := exec.Command("docker", "pull", "-q", imageName).CombinedOutput(); err != nil {
			logger.Debug("docker sandbox: background pull finished with error", "image", imageName, "err", err.Error(), "output", string(bytes.TrimSpace(out)))
		} else {
			logger.Debug("docker sandbox: background pull ok", "image", imageName)
		}
	}()

	dockerArgs := []string{"run", "-d",
		"--name", containerName,
		"--network", "none",
		"--memory", fmt.Sprintf("%dm", s.memoryMB),
		"--cpus", fmt.Sprintf("%f", float64(s.cpuPercent)/100),
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges=true",
		"-w", "/app",
		imageName,
	}
	dockerArgs = append(dockerArgs, getContainerCmd(language)...)

	runCmd := exec.CommandContext(ctx, "docker", dockerArgs...)
	if output, err := runCmd.CombinedOutput(); err != nil {
		logger.Warn("docker sandbox: docker run (create) failed", "container", containerName, "image", imageName, "err", err.Error(), "output", string(bytes.TrimSpace(output)))
		return &DockerResult{Error: fmt.Errorf("failed to create container: %s, %w", string(output), err)}
	}

	copyCmd := exec.Command("docker", "cp", codeFile, fmt.Sprintf("%s:/app/script.%s", containerName, getFileExtension(language)))
	if output, err := copyCmd.CombinedOutput(); err != nil {
		exec.Command("docker", "rm", "-f", containerName).Run()
		logger.Warn("docker sandbox: docker cp failed", "container", containerName, "err", err.Error(), "output", string(bytes.TrimSpace(output)))
		return &DockerResult{Error: fmt.Errorf("failed to copy code: %s, %w", string(output), err)}
	}

	var stdout, stderr bytes.Buffer

	timeoutCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	startCmd := exec.CommandContext(timeoutCtx, "docker", "start", "-a", containerName)
	startCmd.Stdout = &stdout
	startCmd.Stderr = &stderr

	err = startCmd.Run()

	if timeoutCtx.Err() == context.DeadlineExceeded {
		exec.Command("docker", "kill", containerName).Run()
		logger.Warn("docker sandbox: execution timeout", "container", containerName, "limit", s.timeout.String())
		return &DockerResult{Error: fmt.Errorf("execution timeout (>%s)", s.timeout)}
	}

	if stdout.Len() == 0 {
		logsCmd := exec.Command("docker", "logs", "--tail", "100", containerName)
		logsCmd.Stdout = &stdout
		logsCmd.Stderr = &stderr
		logsCmd.Run()
	}

	logger.Info("docker sandbox: execution finished", "container", containerName,
		"stdout_bytes", stdout.Len(), "stderr_bytes", stderr.Len())

	return &DockerResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: 0,
	}
}

func checkDocker() error {
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker not running or not accessible")
	}
	return nil
}

func getFileExtension(language string) string {
	switch language {
	case "python":
		return "py"
	case "javascript":
		return "js"
	case "shell":
		return "sh"
	default:
		return "txt"
	}
}

func getContainerCmd(language string) []string {
	switch language {
	case "python":
		return []string{"python3", "script.py"}
	case "javascript":
		return []string{"node", "script.js"}
	case "shell":
		return []string{"bash", "script.sh"}
	default:
		return []string{"cat"}
	}
}

func generateRunnerCode(language, code, inputJSON string) string {
	switch language {
	case "python":
		return fmt.Sprintf(`
import json
import sys

input_data = json.loads('''%s''')

# User code starts here
%s
`, inputJSON, code)
	case "javascript":
		return fmt.Sprintf(`
const input = %s;

%s
`, inputJSON, code)
	case "shell":
		return code
	default:
		return code
	}
}

func parseDockerImages(imagesJSON string) map[string]string {
	result := make(map[string]string)
	if imagesJSON == "" {
		return result
	}
	var images map[string]string
	if err := json.Unmarshal([]byte(imagesJSON), &images); err != nil {
		logger.Warn("sandbox: failed to parse docker images config", "json", imagesJSON, "err", err.Error())
		return result
	}
	return images
}

func ExecuteShell(code string) (string, error) {
	cmd := exec.Command("bash", "-c", code)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("shell execution failed: %w", err)
	}
	return string(output), nil
}