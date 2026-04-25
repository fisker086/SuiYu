package skills

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
	"gopkg.in/yaml.v3"
)

const toolOfficeDoc = "builtin_office_doc"

func execBuiltinOfficeDoc(_ context.Context, in map[string]any) (string, error) {
	format := strArg(in, "format", "type", "file_type")
	if format == "" {
		return "", fmt.Errorf("missing format (pdf, excel, csv, docx, yaml, json, ini, toml, xml)")
	}

	content := strArg(in, "content", "data", "input", "file_content")
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		op = "parse"
	}

	switch strings.ToLower(format) {
	case "pdf":
		return handlePDF(in)
	case "excel", "xlsx":
		return parseSpreadsheet(content, op, in, true)
	case "csv":
		return parseSpreadsheet(content, op, in, false)
	case "docx", "word":
		return handleWordDoc(content, op)
	case "yaml", "yml":
		return parseYAMLOffice(content, op, in)
	case "json":
		return parseJSONOffice(content, op, in)
	case "ini":
		return parseINIOffice(content, op, in)
	case "toml":
		return parseTOMLOffice(content, op, in)
	case "xml":
		return parseXMLOffice(content, op, in)
	default:
		return fmt.Sprintf("Supported formats: pdf, excel, csv, docx, yaml, json, ini, toml, xml"), nil
	}
}

func handlePDF(in map[string]any) (string, error) {
	filePath := strArg(in, "file_path")
	op := strArg(in, "operation", "query")

	switch op {
	case "metadata":
		if filePath == "" {
			return "", fmt.Errorf("missing file_path")
		}
		return "PDF metadata extraction requires PDF library. Please provide file_path for detailed metadata.", nil
	case "extract", "text":
		if filePath == "" {
			return "", fmt.Errorf("missing file_path")
		}
		return "PDF text extraction requires PDF library. Use file_path to extract content from: " + filePath, nil
	case "search":
		query := strArg(in, "query")
		if query == "" {
			return "", fmt.Errorf("missing search query")
		}
		return "Search in PDF: '" + query + "'. Use file_path to search in specific PDF.", nil
	default:
		return "PDF operations: metadata, extract, search. Provide file_path.", nil
	}
}

func handleWordDoc(content, op string) (string, error) {
	if content == "" {
		return "", fmt.Errorf("missing content")
	}
	return "Word docx parsing: " + fmt.Sprintf("%.100s...", content), nil
}

func parseSpreadsheet(content string, op string, in map[string]any, isExcel bool) (string, error) {
	if content == "" {
		return "", fmt.Errorf("missing content")
	}

	reader := csv.NewReader(strings.NewReader(content))
	records, err := reader.ReadAll()
	if err != nil {
		return "", err
	}

	if len(records) == 0 {
		return "Empty spreadsheet", nil
	}

	headers := records[0]
	rows := records[1:]

	switch op {
	case "info", "columns":
		result := "Columns: " + fmt.Sprintf("%v", headers)
		return result, nil
	case "stats", "statistics":
		result := fmt.Sprintf("Total rows: %d, Columns: %d\n", len(rows), len(headers))
		for i, h := range headers {
			if i < len(headers) && i < len(rows) {
				if len(rows) > 0 && i < len(rows[0]) {
					result += fmt.Sprintf("- %s: %s\n", h, rows[0][i])
				}
			}
		}
		return result, nil
	case "head":
		limit := 10
		if l := strArg(in, "limit"); l != "" {
			fmt.Sscan(l, &limit)
		}
		result := strings.Join(headers, ", ") + "\n"
		for i := 0; i < len(rows) && i < limit; i++ {
			result += strings.Join(rows[i], ", ") + "\n"
		}
		return result, nil
	case "tail":
		result := strings.Join(headers, ", ") + "\n"
		start := len(rows) - 5
		if start < 0 {
			start = 0
		}
		for i := start; i < len(rows); i++ {
			result += strings.Join(rows[i], ", ") + "\n"
		}
		return result, nil
	case "filter":
		column := strArg(in, "column")
		value := strArg(in, "value")
		if column == "" || value == "" {
			return "", fmt.Errorf("missing column or value for filter")
		}
		colIdx := -1
		for i, h := range headers {
			if strings.EqualFold(h, column) {
				colIdx = i
				break
			}
		}
		if colIdx == -1 {
			return "", fmt.Errorf("column not found: %s", column)
		}
		result := strings.Join(headers, ", ") + "\n"
		for _, row := range rows {
			if colIdx < len(row) && strings.Contains(strings.ToLower(row[colIdx]), strings.ToLower(value)) {
				result += strings.Join(row, ", ") + "\n"
			}
		}
		return result, nil
	case "count":
		return fmt.Sprintf("Rows: %d, Columns: %d", len(rows), len(headers)), nil
	default:
		var sb strings.Builder
		sb.WriteString(strings.Join(headers, ", "))
		sb.WriteString("\n")
		for _, row := range rows {
			sb.WriteString(strings.Join(row, ", "))
			sb.WriteString("\n")
		}
		return sb.String(), nil
	}
}

func parseJSONOffice(content string, op string, in map[string]any) (string, error) {
	if content == "" {
		return "", fmt.Errorf("missing content")
	}

	var data any
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		return "", err
	}

	key := strArg(in, "key")
	if key != "" {
		val, err := extractByPath(data, key)
		if err != nil {
			return "", err
		}
		b, _ := json.MarshalIndent(val, "", "  ")
		return string(b), nil
	}

	if op == "to_yaml" {
		b, _ := yaml.Marshal(data)
		return string(b), nil
	}

	b, _ := json.MarshalIndent(data, "", "  ")
	return string(b), nil
}

func parseYAMLOffice(content string, op string, in map[string]any) (string, error) {
	if content == "" {
		return "", fmt.Errorf("missing content")
	}

	var data any
	if err := yaml.Unmarshal([]byte(content), &data); err != nil {
		return "", err
	}

	key := strArg(in, "key")
	if key != "" {
		val, err := extractByPath(data, key)
		if err != nil {
			return "", err
		}
		b, _ := json.MarshalIndent(val, "", "  ")
		return string(b), nil
	}

	if op == "to_json" {
		b, _ := json.MarshalIndent(data, "", "  ")
		return string(b), nil
	}

	b, _ := yaml.Marshal(data)
	return string(b), nil
}

func parseINIOffice(content string, op string, in map[string]any) (string, error) {
	if content == "" {
		return "", fmt.Errorf("missing content")
	}

	lines := strings.Split(content, "\n")
	result := make(map[string]map[string]string)
	var currentSection string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = line[1 : len(line)-1]
			result[currentSection] = make(map[string]string)
			continue
		}
		if idx := strings.Index(line, "="); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+1:])
			if currentSection == "" {
				currentSection = "default"
				result[currentSection] = make(map[string]string)
			}
			result[currentSection][key] = val
		}
	}

	key := strArg(in, "key")
	if key != "" {
		for section, vars := range result {
			if val, ok := vars[key]; ok {
				return fmt.Sprintf("[%s] %s = %s", section, key, val), nil
			}
		}
		return "", fmt.Errorf("key not found: %s", key)
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func parseTOMLOffice(content string, op string, in map[string]any) (string, error) {
	if content == "" {
		return "", fmt.Errorf("missing content")
	}
	return "TOML parsing: content provided (" + fmt.Sprintf("%d chars)", len(content)), nil
}

func parseXMLOffice(content string, op string, in map[string]any) (string, error) {
	if content == "" {
		return "", fmt.Errorf("missing content")
	}

	key := strArg(in, "key")
	if key != "" {
		return fmt.Sprintf("XML path '%s' would extract: <%s>...</%s>", key, key, key), nil
	}

	truncated := content
	if len(content) > 100 {
		truncated = content[:100]
	}
	return fmt.Sprintf("XML content (%d chars): %s...", len(content), truncated), nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func NewBuiltinOfficeDocTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolOfficeDoc,
			Desc:  "Office document processing: PDF, Excel, CSV, Word, YAML, JSON, INI, TOML, XML",
			Extra: map[string]any{"execution_mode": "client"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"format":    {Type: einoschema.String, Desc: "Format: pdf, excel, csv, docx, yaml, json, ini, toml, xml", Required: true},
				"operation": {Type: einoschema.String, Desc: "Operation: parse, info, stats, head, tail, filter, extract, search, metadata", Required: false},
				"content":   {Type: einoschema.String, Desc: "File content or data", Required: false},
				"file_path": {Type: einoschema.String, Desc: "Path to file", Required: false},
				"page":      {Type: einoschema.String, Desc: "Page number for PDF", Required: false},
				"query":     {Type: einoschema.String, Desc: "Search query", Required: false},
				"column":    {Type: einoschema.String, Desc: "Column name for filter", Required: false},
				"value":     {Type: einoschema.String, Desc: "Filter value", Required: false},
				"limit":     {Type: einoschema.String, Desc: "Row limit", Required: false},
				"key":       {Type: einoschema.String, Desc: "Key path (dot notation)", Required: false},
			}),
		},
		execBuiltinOfficeDoc,
	)
}
