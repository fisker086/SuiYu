package storage

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// safeDatabaseName restricts auto-created database names to unquoted PostgreSQL identifiers.
var safeDatabaseName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// EnsureDatabaseExists connects to the maintenance database "postgres" and creates the
// database named in dsn if it does not exist. Skips when the target is empty or "postgres".
// The name must match safeDatabaseName; otherwise returns an error (create manually).
// Requires CREATEDB privilege or superuser for the user in dsn.
func EnsureDatabaseExists(ctx context.Context, dsn string) error {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("parse database url: %w", err)
	}
	targetDB := strings.TrimSpace(cfg.Database)
	if targetDB == "" || targetDB == "postgres" {
		return nil
	}
	if !safeDatabaseName.MatchString(targetDB) {
		return fmt.Errorf("database name %q must match [a-zA-Z_][a-zA-Z0-9_]* for auto-create, or create the database manually", targetDB)
	}

	cfg.Database = "postgres"
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("connect to postgres maintenance db: %w", err)
	}
	defer conn.Close(ctx)

	var exists bool
	if err := conn.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)`, targetDB,
	).Scan(&exists); err != nil {
		return fmt.Errorf("check database exists: %w", err)
	}
	if exists {
		return nil
	}

	q := fmt.Sprintf(`CREATE DATABASE %s`, quoteIdent(targetDB))
	if _, err := conn.Exec(ctx, q); err != nil {
		var pe *pgconn.PgError
		if errors.As(err, &pe) && pe.Code == "42P04" {
			return nil
		}
		return fmt.Errorf("create database %q: %w", targetDB, err)
	}
	return nil
}

func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
