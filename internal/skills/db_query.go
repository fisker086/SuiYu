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

const toolDBQuery = "builtin_db_query"

var blockedSQLKeywords = []string{
	"INSERT", "UPDATE", "DELETE", "DROP", "ALTER", "CREATE",
	"TRUNCATE", "GRANT", "REVOKE", "REPLACE", "MERGE",
}

// normalizeSQLDriver maps common names to registered database/sql drivers:
// - postgres / postgresql -> pgx (github.com/jackc/pgx/v5/stdlib)
// - mysql unchanged (github.com/go-sql-driver/mysql)
func normalizeSQLDriver(driver string) (string, error) {
	d := strings.ToLower(strings.TrimSpace(driver))
	switch d {
	case "":
		return "", fmt.Errorf("missing driver (mysql or postgres)")
	case "postgres", "postgresql":
		return "pgx", nil
	case "pgx", "pgx/v5", "mysql":
		return d, nil
	default:
		return "", fmt.Errorf("unsupported driver %q (use mysql, postgres, pgx)", driver)
	}
}

func execBuiltinDBQuery(_ context.Context, in map[string]any) (string, error) {
	rawDriver := strArg(in, "driver", "db_type", "database_type")
	driver, err := normalizeSQLDriver(rawDriver)
	if err != nil {
		return "", err
	}

	dsn := strArg(in, "dsn", "connection_string", "conn")
	if dsn == "" {
		return "", fmt.Errorf("missing DSN (connection string)")
	}

	query := strArg(in, "query", "sql", "statement")
	if query == "" {
		return "", fmt.Errorf("missing query")
	}

	if !isSafeSQLQuery(query) {
		return "", fmt.Errorf("query contains blocked keywords (SELECT and EXPLAIN only)")
	}

	timeoutSec := strArg(in, "timeout", "timeout_seconds")
	timeout := 10 * time.Second
	if timeoutSec != "" {
		var t int
		if _, err := fmt.Sscanf(timeoutSec, "%d", &t); err == nil && t > 0 {
			timeout = time.Duration(t) * time.Second
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return "", fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return "", fmt.Errorf("failed to ping database: %w", err)
	}

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return "", fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

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
		return fmt.Sprintf("Query returned 0 rows. Columns: %s", strings.Join(columns, ", ")), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Query returned %d rows, %d columns:\n\n", len(results), len(columns)))
	sb.WriteString(strings.Join(columns, " | "))
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("-", 80))
	sb.WriteString("\n")

	for _, row := range results {
		var vals []string
		for _, col := range columns {
			vals = append(vals, fmt.Sprint(row[col]))
		}
		sb.WriteString(strings.Join(vals, " | "))
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

func isSafeSQLQuery(query string) bool {
	upper := strings.ToUpper(strings.TrimSpace(query))

	if !strings.HasPrefix(upper, "SELECT") && !strings.HasPrefix(upper, "EXPLAIN") {
		return false
	}

	for _, keyword := range blockedSQLKeywords {
		if strings.Contains(upper, keyword) {
			return false
		}
	}

	return true
}

func NewBuiltinDBQueryTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name: toolDBQuery,
			Desc: "Read-only database query tool supporting MySQL and PostgreSQL. Only SELECT and EXPLAIN queries are allowed.",
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"driver":  {Type: einoschema.String, Desc: "mysql, postgres (alias for pgx), or pgx", Required: true},
				"dsn":     {Type: einoschema.String, Desc: "DSN: Postgres e.g. postgres://user:pass@host:5432/db?sslmode=disable; MySQL e.g. user:pass@tcp(host:3306)/dbname", Required: true},
				"query":   {Type: einoschema.String, Desc: "SQL query to execute (SELECT only)", Required: true},
				"timeout": {Type: einoschema.String, Desc: "Query timeout in seconds (default: 10)", Required: false},
			}),
		},
		execBuiltinDBQuery,
	)
}
