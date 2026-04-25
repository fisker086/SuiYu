package storage

import (
	"github.com/fisk086/sya/internal/logger"
	"github.com/jackc/pgx/v5"
)

// LogDatabaseTarget logs host, port, database, and user from a PostgreSQL DSN (never logs password).
func LogDatabaseTarget(dsn string) {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		logger.Warn("database dsn parse failed (check DATABASE_URL)", "err", err)
		return
	}
	logger.Info("database target", "host", cfg.Host, "port", cfg.Port, "database", cfg.Database, "user", cfg.User)
}
