---
name: mysql-explain
description: MySQL EXPLAIN query analyzer: supports EXPLAIN, EXPLAIN ANALYZE, EXPLAIN FORMAT. Shows query execution plan, index usage, and estimated costs.
activation_keywords: [mysql, explain, analyze, query, sql, plan, index, explain format]
execution_mode: server
---

# MySQL Explain Skill

Provides MySQL query execution plan analysis:
- **EXPLAIN**: Show query execution plan
- **EXPLAIN ANALYZE**: Show actual execution time and costs (MySQL 8.0+)
- **EXPLAIN FORMAT**: Output in JSON, TREE, or traditional format
- **EXPLAIN PARTITIONS**: Show partition information

Shows:
- Access type (ALL, index, range, ref, eq_ref, const, etc.)
- Key used and possible keys
- Rows examined
- Extra info (using filesort, using index, etc.)

Use `builtin_mysql_explain` tool with fields:
- `dsn`: MySQL DSN (e.g., user:pass@tcp(host:3306)/dbname)
- `query`: SQL query to explain
- `type`: Explain type (classic, analyze, format, partitions)
- `format`: Output format for FORMAT type (json, tree, traditional)
- `timeout`: Query timeout in seconds (default: 30)
- `show_execution_time`: Show actual execution time (true/1 or false)

Example:
```
dsn: "root:password@tcp(localhost:3306)/mydb"
query: "SELECT * FROM users WHERE email = 'test@example.com'"
type: "analyze"
```

Note: This tool executes EXPLAIN on the query, not the query itself. Use `show_execution_time` to also run and time the actual query.