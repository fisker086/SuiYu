---
name: sql-explain
description: Interpret EXPLAIN / query plans (MySQL, PostgreSQL, or pasted plan text): access types, indexes, and tuning hints.
activation_keywords: [sql, explain, query plan, execution plan, slow query, index, postgres, mysql]
execution_mode: server
---

# SQL Explain Skill

Use when the user pastes **EXPLAIN** output or asks how to read a **query plan**.

- **MySQL live analysis**: Prefer the **MySQL Explain** skill (`builtin_mysql_explain` / `mysql_explain`) when you need to run `EXPLAIN` / `EXPLAIN ANALYZE` against a DSN.
- **Any engine (text)**: Parse pasted plans (MySQL traditional/JSON/TREE, PostgreSQL `EXPLAIN`, etc.) and explain:
  - Access type (seq scan vs index, join order hints)
  - Whether indexes are used as intended
  - “Using filesort”, “Using temporary”, buffer usage (when present)
  - Practical next steps (indexes to add/change, query rewrite)

If the plan is truncated, ask only for the missing fragment.
