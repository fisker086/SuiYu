package skills

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
)

const toolTestReportParser = "builtin_test_report_parser"

type JUnitTestCase struct {
	XMLName   xml.Name `xml:"testcase"`
	Name      string   `xml:"name,attr"`
	ClassName string   `xml:"classname,attr"`
	Time      string   `xml:"time,attr"`
	Failure   string   `xml:"failure"`
}

type JUnitTestSuite struct {
	XMLName   xml.Name        `xml:"testsuite"`
	Tests     int             `xml:"tests,attr"`
	Failures  int             `xml:"failures,attr"`
	Skipped   int             `xml:"skipped,attr"`
	Errors    int             `xml:"errors,attr"`
	Time      string          `xml:"time,attr"`
	Name      string          `xml:"name,attr"`
	TestCases []JUnitTestCase `xml:"testcase"`
}

type JUnitTestSuites struct {
	XMLName   xml.Name         `xml:"testsuites"`
	TestSuite []JUnitTestSuite `xml:"testsuite"`
}

func execBuiltinTestReportParser(_ context.Context, in map[string]any) (string, error) {
	format := strArg(in, "format", "type")
	if format == "" {
		format = "junit"
	}
	format = strings.ToLower(format)

	path := strArg(in, "path", "file", "report")
	if path == "" {
		return "", fmt.Errorf("missing path")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	op := strArg(in, "operation", "op", "action")
	if op == "" {
		op = "parse"
	}

	filter := strArg(in, "filter", "status")

	switch format {
	case "junit":
		return parseJUnitReport(string(data), op, filter)
	case "json":
		return parseJSONReport(string(data), op, filter)
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}
}

func parseJUnitReport(data string, op, filter string) (string, error) {
	var suites JUnitTestSuites
	if err := xml.Unmarshal([]byte(data), &suites); err != nil {
		return "", fmt.Errorf("failed to parse XML: %w", err)
	}

	var totalTests, totalFailures, totalSkipped int
	var failedCases []JUnitTestCase

	for _, s := range suites.TestSuite {
		totalTests += s.Tests
		totalFailures += s.Failures
		totalSkipped += s.Skipped

		for _, tc := range s.TestCases {
			if tc.Failure != "" {
				failedCases = append(failedCases, tc)
			}
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Test Summary:\n"))
	sb.WriteString(fmt.Sprintf("  Total: %d\n", totalTests))
	sb.WriteString(fmt.Sprintf("  Passed: %d\n", totalTests-totalFailures-totalSkipped))
	sb.WriteString(fmt.Sprintf("  Failed: %d\n", totalFailures))
	sb.WriteString(fmt.Sprintf("  Skipped: %d\n", totalSkipped))

	if op == "failures" && len(failedCases) > 0 {
		sb.WriteString("\nFailed Tests:\n")
		for _, tc := range failedCases {
			sb.WriteString(fmt.Sprintf("  - %s (%s): %s\n", tc.Name, tc.ClassName, tc.Failure))
		}
	}

	return sb.String(), nil
}

func parseJSONReport(data string, op, filter string) (string, error) {
	var result map[string]any
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		return "", fmt.Errorf("failed to parse JSON: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("Test Report:\n")

	if numTests, ok := result["numTests"].(float64); ok {
		sb.WriteString(fmt.Sprintf("  Tests: %.0f\n", numTests))
	}
	if passed, ok := result["passed"].(float64); ok {
		sb.WriteString(fmt.Sprintf("  Passed: %.0f\n", passed))
	}
	if failed, ok := result["failed"].(float64); ok {
		sb.WriteString(fmt.Sprintf("  Failed: %.0f\n", failed))
	}
	if duration, ok := result["duration"].(float64); ok {
		sb.WriteString(fmt.Sprintf("  Duration: %.2fs\n", duration))
	}

	return sb.String(), nil
}

func NewBuiltinTestReportParserTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name: toolTestReportParser,
			Desc: "Parse and analyze test results from JUnit XML and JSON formats.",
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"format": {Type: einoschema.String, Desc: "Report format: junit, json", Required: false},
				"path":   {Type: einoschema.String, Desc: "Path to test report file", Required: true},
				"op":     {Type: einoschema.String, Desc: "Operation: parse, summary, failures", Required: false},
				"filter": {Type: einoschema.String, Desc: "Filter by status: passed, failed, skipped", Required: false},
			}),
		},
		execBuiltinTestReportParser,
	)
}
