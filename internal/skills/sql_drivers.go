package skills

// Register database/sql drivers for builtin_db_query. Without these imports,
// sql.Open("postgres"|"mysql", ...) fails with unknown driver at runtime.
import (
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)
