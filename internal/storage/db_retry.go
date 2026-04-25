package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/fisk086/sya/internal/logger"
)

// isTransientDBErr reports dial / startup errors that often resolve after a short wait (e.g. compose race).
func isTransientDBErr(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "connection refused") ||
		strings.Contains(s, "connection reset") ||
		strings.Contains(s, "i/o timeout") ||
		(strings.Contains(s, "timeout") && strings.Contains(s, "dial"))
}

// ConnectPostgresWithRetry runs EnsureDatabaseExists then NewPostgresStorage, retrying transient errors.
func ConnectPostgresWithRetry(ctx context.Context, dsn string, dimension int) (*PostgresStorage, error) {
	const maxAttempts = 30
	const delay = 2 * time.Second
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := EnsureDatabaseExists(ctx, dsn); err != nil {
			lastErr = err
			if !isTransientDBErr(err) {
				return nil, err
			}
			logger.Info("postgres not ready (ensure_database)", "attempt", attempt, "max", maxAttempts, "err", err)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
			continue
		}
		pg, err := NewPostgresStorage(ctx, dsn, dimension)
		if err == nil {
			if attempt > 1 {
				logger.Info("postgres connected after retry", "attempts", attempt)
			}
			return pg, nil
		}
		lastErr = err
		if !isTransientDBErr(err) {
			return nil, err
		}
		logger.Info("postgres not ready (pool)", "attempt", attempt, "max", maxAttempts, "err", err)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
	return nil, fmt.Errorf("postgres after %d attempts: %w", maxAttempts, lastErr)
}
