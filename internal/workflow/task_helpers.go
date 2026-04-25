package workflow

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/fisk086/sya/internal/agent"
)

var agentRuntime *agent.Runtime

func SetAgentRuntime(rt *agent.Runtime) {
	agentRuntime = rt
}

type ToolFunc func(ctx context.Context, input string) (string, error)

var toolRegistry = make(map[string]ToolFunc)

func RegisterTool(name string, fn ToolFunc) {
	toolRegistry[name] = fn
}

func GetToolRegistry() map[string]ToolFunc {
	return toolRegistry
}

func SetToolRegistry(registry map[string]ToolFunc) {
	toolRegistry = registry
}

func resolveTemplateValue(tmpl string, variables map[string]any, nodeOutputs map[string]any) string {
	re := regexp.MustCompile(`\{\{(\.?[\w.]+)\}\}`)
	return re.ReplaceAllStringFunc(tmpl, func(match string) string {
		submatches := re.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}
		keyPath := strings.TrimPrefix(submatches[1], ".")

		parts := strings.Split(keyPath, ".")
		if len(parts) == 0 {
			return match
		}

		var val any
		var ok bool

		val, ok = variables[parts[0]]
		if !ok {
			val, ok = nodeOutputs[parts[0]]
		}
		if !ok {
			return match
		}

		for i := 1; i < len(parts) && val != nil; i++ {
			val, ok = getNestedField(val, parts[i])
			if !ok {
				return match
			}
		}

		if val == nil {
			return match
		}

		if str, ok := val.(string); ok {
			return str
		}
		return fmt.Sprintf("%v", val)
	})
}

func getNestedField(data any, field string) (any, bool) {
	switch v := data.(type) {
	case map[string]any:
		if val, ok := v[field]; ok {
			return val, true
		}
	case []any:
		if idx := parseIndex(field, len(v)); idx >= 0 && idx < len(v) {
			return v[idx], true
		}
	}
	return nil, false
}

func parseIndex(s string, length int) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return -1
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func doHTTPRequest(ctx context.Context, method, url string, headers map[string]string, body string, timeoutMs int) (string, int, error) {
	client := &http.Client{
		Timeout: time.Duration(timeoutMs) * time.Millisecond,
	}

	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return "", 0, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	if method != "GET" && method != "HEAD" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", resp.StatusCode, err
	}

	return string(respBody), resp.StatusCode, nil
}

func executeCodeRuntime(ctx context.Context, language, code string, input map[string]any) (string, error) {
	switch language {
	case "python":
		return executePython(ctx, code, input)
	case "javascript":
		return executeJavaScript(ctx, code, input)
	default:
		return "", fmt.Errorf("unsupported language: %s", language)
	}
}

func executePython(ctx context.Context, code string, input map[string]any) (string, error) {
	inputJSON := mapToJSON(input)

	script := fmt.Sprintf(`
import json
import sys

input_data = %s

# User code starts here
%s
`, inputJSON, code)

	cmd := exec.CommandContext(ctx, "python3", "-c", script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return stderr.String(), fmt.Errorf("python execution failed: %w", err)
	}

	return stdout.String(), nil
}

func executeJavaScript(ctx context.Context, code string, input map[string]any) (string, error) {
	inputJSON := mapToJSON(input)

	script := fmt.Sprintf(`
const input = %s;

%s
`, inputJSON, code)

	cmd := exec.CommandContext(ctx, "node", "-e", script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return stderr.String(), fmt.Errorf("node execution failed: %w", err)
	}

	return stdout.String(), nil
}

func mapToJSON(m map[string]any) string {
	if len(m) == 0 {
		return "{}"
	}
	var parts []string
	for k, v := range m {
		parts = append(parts, fmt.Sprintf("%q: %v", k, formatValue(v)))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func formatValue(v any) string {
	switch val := v.(type) {
	case string:
		return strconv.Quote(val)
	case nil:
		return "null"
	case bool:
		return fmt.Sprintf("%t", val)
	case int, int64, float64:
		return fmt.Sprintf("%v", val)
	case map[string]any:
		return mapToJSON(val)
	case []any:
		var parts []string
		for _, item := range val {
			parts = append(parts, formatValue(item))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	default:
		return strconv.Quote(fmt.Sprintf("%v", val))
	}
}

func evaluateConditionExpr(condition string, input *TaskInput) bool {
	condition = strings.TrimSpace(condition)

	if strings.HasPrefix(condition, "{{.") && strings.HasSuffix(condition, "}}") {
		key := strings.TrimSuffix(strings.TrimPrefix(condition, "{{."), "}}")
		val, ok := input.Variables[key]
		if !ok {
			return false
		}
		switch v := val.(type) {
		case bool:
			return v
		case string:
			return v != "" && v != "false" && v != "0"
		case nil:
			return false
		default:
			return true
		}
	}

	resolved := resolveTemplate(condition, input)

	if resolved == "true" || resolved == "1" || resolved == "yes" {
		return true
	}
	if resolved == "false" || resolved == "0" || resolved == "no" || resolved == "" {
		return false
	}

	if strings.Contains(resolved, "==") {
		parts := strings.SplitN(resolved, "==", 2)
		if len(parts) == 2 {
			return strings.TrimSpace(parts[0]) == strings.TrimSpace(parts[1])
		}
	}

	if strings.Contains(resolved, "!=") {
		parts := strings.SplitN(resolved, "!=", 2)
		if len(parts) == 2 {
			return strings.TrimSpace(parts[0]) != strings.TrimSpace(parts[1])
		}
	}

	return resolved != ""
}
