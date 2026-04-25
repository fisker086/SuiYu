package skills

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
)

const toolMySQLExplain = "builtin_mysql_explain"

func execBuiltinMySQLExplain(_ context.Context, in map[string]any) (string, error) {
	dsn := strArg(in, "dsn", "connection", "conn")
	if dsn == "" {
		return "", fmt.Errorf("missing DSN (e.g., user:pass@tcp(host:3306)/dbname)")
	}

	query := strArg(in, "query", "sql", "statement")
	if query == "" {
		return "", fmt.Errorf("missing query to explain")
	}

	explainType := strArg(in, "type", "explain_type")
	if explainType == "" {
		explainType = "analyze"
	}

	timeoutSec := strArg(in, "timeout", "timeout_seconds")
	timeout := 30 * time.Second
	if timeoutSec != "" {
		var t int
		if _, err := fmt.Sscanf(timeoutSec, "%d", &t); err == nil && t > 0 {
			timeout = time.Duration(t) * time.Second
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return "", fmt.Errorf("failed to connect to MySQL: %w", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return "", fmt.Errorf("failed to ping MySQL: %w", err)
	}

	var result string

	switch explainType {
	case "analyze":
		rows, err := db.QueryContext(ctx, "EXPLAIN ANALYZE "+query)
		if err != nil {
			return "", fmt.Errorf("explain analyze failed: %w", err)
		}
		defer rows.Close()

		result, err = formatExplainRows(rows, "EXPLAIN ANALYZE")
		if err != nil {
			return "", err
		}

	case "format":
		format := strArg(in, "format", "output_format")
		if format == "" {
			format = "json"
		}
		rows, err := db.QueryContext(ctx, "EXPLAIN FORMAT="+format+" "+query)
		if err != nil {
			return "", fmt.Errorf("explain format failed: %w", err)
		}
		defer rows.Close()

		result, err = formatExplainRows(rows, "EXPLAIN FORMAT="+format)
		if err != nil {
			return "", err
		}

	case "classic":
		rows, err := db.QueryContext(ctx, "EXPLAIN "+query)
		if err != nil {
			return "", fmt.Errorf("explain failed: %w", err)
		}
		defer rows.Close()

		result, err = formatExplainRows(rows, "EXPLAIN")
		if err != nil {
			return "", err
		}

	case "partitions":
		rows, err := db.QueryContext(ctx, "EXPLAIN PARTITIONS "+query)
		if err != nil {
			return "", fmt.Errorf("explain partitions failed: %w", err)
		}
		defer rows.Close()

		result, err = formatExplainRows(rows, "EXPLAIN PARTITIONS")
		if err != nil {
			return "", err
		}

	default:
		rows, err := db.QueryContext(ctx, "EXPLAIN "+query)
		if err != nil {
			return "", fmt.Errorf("explain failed: %w", err)
		}
		defer rows.Close()

		result, err = formatExplainRows(rows, "EXPLAIN")
		if err != nil {
			return "", err
		}
	}

	execTime := strArg(in, "show_execution_time", "timing")
	if execTime == "true" || execTime == "1" {
		start := time.Now()
		_, err = db.ExecContext(ctx, query)
		elapsed := time.Since(start)
		if err == nil {
			result += fmt.Sprintf("\n\nExecution time: %v", elapsed)
		}
	}

	return result, nil
}

func formatExplainRows(rows *sql.Rows, title string) (string, error) {
	columns, err := rows.Columns()
	if err != nil {
		return "", fmt.Errorf("failed to get columns: %w", err)
	}

	var results []map[string]any
	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return "", fmt.Errorf("failed to scan row: %w", err)
		}

		row := make(map[string]any)
		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("row iteration: %w", err)
	}

	if len(results) == 0 {
		return fmt.Sprintf("%s: (no rows)", title), nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s:\n\n", title))

	allKeys := make(map[string]bool)
	for _, r := range results {
		for k := range r {
			allKeys[k] = true
		}
	}

	colWidths := make(map[string]int)
	for k := range allKeys {
		colWidths[k] = len(k)
	}
	for _, r := range results {
		for k, v := range r {
			l := len(fmt.Sprint(v))
			if l > colWidths[k] {
				colWidths[k] = l
			}
		}
	}

	for _, k := range columns {
		b.WriteString(k)
		b.WriteString(strings.Repeat(" ", colWidths[k]-len(k)+2))
	}
	b.WriteString("\n")

	for _, k := range columns {
		b.WriteString(strings.Repeat("-", colWidths[k]))
		b.WriteString("  ")
	}
	b.WriteString("\n")

	for _, r := range results {
		for _, k := range columns {
			v := r[k]
			s := fmt.Sprint(v)
			b.WriteString(s)
			b.WriteString(strings.Repeat(" ", colWidths[k]-len(s)+2))
		}
		b.WriteString("\n")
	}

	if len(results) > 1 {
		b.WriteString(fmt.Sprintf("\n(%d rows)", len(results)))
	}

	return b.String(), nil
}

func NewBuiltinMySQLExplainTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name: toolMySQLExplain,
			Desc: "MySQL EXPLAIN query analyzer: supports EXPLAIN, EXPLAIN ANALYZE, EXPLAIN FORMAT (JSON/TREE/VERBOSE). Shows query execution plan, index usage, and estimated costs.",
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"dsn":                 {Type: einoschema.String, Desc: "MySQL DSN: user:pass@tcp(host:3306)/dbname", Required: true},
				"query":               {Type: einoschema.String, Desc: "SQL query to explain", Required: true},
				"type":                {Type: einoschema.String, Desc: "Explain type: classic, analyze, format, partitions (default: analyze)", Required: false},
				"format":              {Type: einoschema.String, Desc: "Output format for FORMAT type: json, tree, traditional (default: json)", Required: false},
				"timeout":             {Type: einoschema.String, Desc: "Query timeout in seconds (default: 30)", Required: false},
				"show_execution_time": {Type: einoschema.String, Desc: "Show actual execution time (true/1 or false)", Required: false},
			}),
		},
		execBuiltinMySQLExplain,
	)
}
