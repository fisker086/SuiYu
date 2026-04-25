package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
)

const toolLogAnalyzer = "builtin_log_analyzer"

var (
	nginxAccessRe = regexp.MustCompile(`^(?P<ip>\S+) \S+ \S+ \[(?P<time>[^\]]+)\] "(?P<method>\S+) (?P<path>\S+) \S+" (?P<status>\d+) (?P<size>\d+)`)
	syslogRe      = regexp.MustCompile(`^(?P<month>\w+)\s+(?P<day>\d+)\s+(?P<time>\S+)\s+(?P<host>\S+)\s+(?P<process>\S+?)(?:\[(?P<pid>\d+)\])?:\s+(?P<message>.*)$`)
	jsonLogRe     = regexp.MustCompile(`^\{.*"level"\s*:\s*"(?P<level>\w+)".*\}$`)
)

func execBuiltinLogAnalyzer(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		op = "parse"
	}

	content := strArg(in, "log_content", "content", "logs", "input")
	if content == "" {
		return "", fmt.Errorf("missing log content")
	}

	format := strArg(in, "format", "log_format", "type")
	if format == "" {
		format = "auto"
	}

	lines := strings.Split(content, "\n")
	lines = filterEmpty(lines)

	switch op {
	case "parse":
		return parseLogs(lines, format)

	case "filter":
		level := strArg(in, "filter_level", "level", "severity")
		if level == "" {
			return "", fmt.Errorf("missing filter level for filter operation")
		}
		return filterLogsByLevel(lines, format, level)

	case "summarize":
		return summarizeLogs(lines, format)

	default:
		return "", fmt.Errorf("unknown log_analyzer operation: %s", op)
	}
}

func detectFormat(lines []string) string {
	if len(lines) == 0 {
		return "unknown"
	}

	sample := lines[0]

	if jsonLogRe.MatchString(sample) {
		return "json"
	}
	if nginxAccessRe.MatchString(sample) {
		return "nginx"
	}
	if syslogRe.MatchString(sample) {
		return "syslog"
	}

	if strings.Contains(sample, " - - [") {
		return "nginx"
	}
	if strings.Contains(sample, "[error]") || strings.Contains(sample, "[warn]") {
		return "nginx"
	}

	return "unknown"
}

func parseLogs(lines []string, format string) (string, error) {
	if format == "auto" {
		format = detectFormat(lines)
	}

	var parsed []map[string]string

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := parseLogLine(line, format)
		if fields == nil {
			fields = map[string]string{"raw": line, "_parse_error": "unmatched"}
		}
		fields["_line"] = fmt.Sprintf("%d", i+1)
		parsed = append(parsed, fields)
	}

	if len(parsed) == 0 {
		return "No log lines to parse", nil
	}

	result := fmt.Sprintf("Format detected: %s\nParsed %d lines:\n\n", format, len(parsed))
	for _, fields := range parsed {
		for k, v := range fields {
			result += fmt.Sprintf("  %s: %s\n", k, v)
		}
		result += "\n"
	}

	return result, nil
}

func parseLogLine(line string, format string) map[string]string {
	switch format {
	case "nginx":
		return parseNginxLog(line)
	case "syslog":
		return parseSyslog(line)
	case "json":
		return parseJSONLog(line)
	default:
		if m := parseNginxLog(line); m != nil {
			return m
		}
		if m := parseSyslog(line); m != nil {
			return m
		}
		if m := parseJSONLog(line); m != nil {
			return m
		}
		return nil
	}
}

func parseNginxLog(line string) map[string]string {
	matches := nginxAccessRe.FindStringSubmatch(line)
	if matches == nil {
		return nil
	}

	fields := make(map[string]string)
	for i, name := range nginxAccessRe.SubexpNames() {
		if i != 0 && name != "" {
			fields[name] = matches[i]
		}
	}
	return fields
}

func parseSyslog(line string) map[string]string {
	matches := syslogRe.FindStringSubmatch(line)
	if matches == nil {
		return nil
	}

	fields := make(map[string]string)
	for i, name := range syslogRe.SubexpNames() {
		if i != 0 && name != "" {
			fields[name] = matches[i]
		}
	}
	return fields
}

func parseJSONLog(line string) map[string]string {
	var obj map[string]any
	if err := jsonUnmarshal(line, &obj); err != nil {
		return nil
	}

	fields := make(map[string]string)
	for k, v := range obj {
		fields[k] = fmt.Sprint(v)
	}
	return fields
}

func jsonUnmarshal(s string, v any) error {
	return json.Unmarshal([]byte(s), v)
}

func filterLogsByLevel(lines []string, format string, level string) (string, error) {
	if format == "auto" {
		format = detectFormat(lines)
	}

	level = strings.ToLower(level)
	var matched []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if matchesLogLevel(line, level) {
			matched = append(matched, line)
		}
	}

	result := fmt.Sprintf("Filtering logs for level: %s\nMatched %d of %d lines:\n\n", level, len(matched), len(lines))
	for _, m := range matched {
		result += m + "\n"
	}

	return result, nil
}

func matchesLogLevel(line string, level string) bool {
	lineLower := strings.ToLower(line)

	levelMap := map[string][]string{
		"debug": {"debug", "dbg", "trace"},
		"info":  {"info", "notice", "information"},
		"warn":  {"warn", "warning", "deprec"},
		"error": {"error", "err", "fail", "fatal", "crit", "critical", "alert", "emerg", "panic"},
	}

	targetLevels, ok := levelMap[level]
	if !ok {
		targetLevels = []string{level}
	}

	for _, lvl := range targetLevels {
		if strings.Contains(lineLower, lvl) {
			return true
		}
	}

	return false
}

func summarizeLogs(lines []string, format string) (string, error) {
	if format == "auto" {
		format = detectFormat(lines)
	}

	totalLines := 0
	errorCount := 0
	warnCount := 0
	infoCount := 0
	ipCounts := make(map[string]int)
	statusCounts := make(map[string]int)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		totalLines++

		lower := strings.ToLower(line)
		if strings.Contains(lower, "error") || strings.Contains(lower, "err") || strings.Contains(lower, "fatal") || strings.Contains(lower, "crit") {
			errorCount++
		} else if strings.Contains(lower, "warn") {
			warnCount++
		} else if strings.Contains(lower, "info") || strings.Contains(lower, "notice") {
			infoCount++
		}

		fields := parseLogLine(line, format)
		if fields != nil {
			if ip, ok := fields["ip"]; ok {
				ipCounts[ip]++
			}
			if status, ok := fields["status"]; ok {
				statusCounts[status]++
			}
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Log Summary (format: %s)\n", format))
	sb.WriteString(fmt.Sprintf("Total lines: %d\n", totalLines))
	sb.WriteString(fmt.Sprintf("Errors: %d\n", errorCount))
	sb.WriteString(fmt.Sprintf("Warnings: %d\n", warnCount))
	sb.WriteString(fmt.Sprintf("Info: %d\n", infoCount))

	if len(ipCounts) > 0 {
		sb.WriteString("\nTop IPs:\n")
		for ip, count := range topN(ipCounts, 5) {
			sb.WriteString(fmt.Sprintf("  %s: %d requests\n", ip, count))
		}
	}

	if len(statusCounts) > 0 {
		sb.WriteString("\nStatus codes:\n")
		for status, count := range topN(statusCounts, 10) {
			sb.WriteString(fmt.Sprintf("  %s: %d responses\n", status, count))
		}
	}

	return sb.String(), nil
}

func topN(m map[string]int, n int) map[string]int {
	type kv struct {
		Key   string
		Value int
	}

	var s []kv
	for k, v := range m {
		s = append(s, kv{k, v})
	}

	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if s[j].Value > s[i].Value {
				s[i], s[j] = s[j], s[i]
			}
		}
	}

	result := make(map[string]int)
	for i := 0; i < n && i < len(s); i++ {
		result[s[i].Key] = s[i].Value
	}
	return result
}

func filterEmpty(lines []string) []string {
	var result []string
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			result = append(result, l)
		}
	}
	return result
}

func NewBuiltinLogAnalyzerTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolLogAnalyzer,
			Desc:  "Parse, filter, and summarize logs in common formats (nginx, syslog, JSON, Apache).",
			Extra: map[string]any{"execution_mode": "client"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation":    {Type: einoschema.String, Desc: "Operation: parse, filter, summarize", Required: false},
				"log_content":  {Type: einoschema.String, Desc: "Log content to analyze", Required: true},
				"format":       {Type: einoschema.String, Desc: "Log format: nginx, syslog, json, apache, auto", Required: false},
				"filter_level": {Type: einoschema.String, Desc: "Filter level: debug, info, warn, error (for filter operation)", Required: false},
			}),
		},
		execBuiltinLogAnalyzer,
	)
}
