---
name: database-query
description: Read-only database queries for MySQL and PostgreSQL inspection
activation_keywords: [database, db, sql, query, mysql, postgresql, postgres, table, select]
execution_mode: server
---

# Database Query Skill

Provides read-only database query capabilities:

- Execute SELECT queries on MySQL and PostgreSQL
- Inspect table schemas and indexes
- Check database statistics and connection info
- Analyze query performance (`EXPLAIN ...` only when the whole statement is safe; avoid `EXPLAIN` on DDL)

Use `builtin_db_query` tool with fields:

- `driver`: `mysql`, or `postgres` / `postgresql` / `pgx` (on the **AgentSphere desktop**, MySQL/PostgreSQL use embedded Rust drivers; you do **not** need `psql` or `mysql` installed on PATH)
- `dsn`: connection string, for example:
  - PostgreSQL: `postgres://USER:PASSWORD@HOST:5432/DATABASE`（桌面端若未写 `sslmode` 会自动补上 `sslmode=disable`；若显式要求 SSL 如 `require`/`verify-full`，桌面工具会提示不支持，请改用其他客户端）
  - MySQL: `USER:PASSWORD@tcp(HOST:3306)/DATABASE`
- `query`: SQL to execute (SELECT or EXPLAIN-only; see safety note below)
- `timeout`: optional, query timeout in seconds (default: 10)

Note: Only SELECT and EXPLAIN forms that do not contain blocked substrings are allowed (INSERT, UPDATE, DELETE, DROP, ALTER, CREATE, etc. anywhere in the string will be rejected). INSERT, UPDATE, DELETE, DROP, ALTER, CREATE are blocked for safety.

Wrong DSN, blocked SQL, or TLS mismatch (e.g. server requires SSL but DSN uses `sslmode=disable`) will return an error in the tool result.
