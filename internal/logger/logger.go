// Package logger configures the process-wide slog logger. Call Init once at startup
// (after godotenv, before other work) and use Info/Warn/Error/Debug/Fatal or log/slog
// against the default logger elsewhere.
package logger

import (
	"log/slog"
	"os"
	"strings"
)

// Init sets slog.Default from LOG_LEVEL and LOG_FORMAT. Defaults: info, text.
func Init() {
	lvl := parseLevel(os.Getenv("LOG_LEVEL"))
	format := strings.ToLower(strings.TrimSpace(os.Getenv("LOG_FORMAT")))
	if format == "" {
		format = "text"
	}

	opts := &slog.HandlerOptions{Level: lvl}
	var h slog.Handler
	switch format {
	case "json":
		h = slog.NewJSONHandler(os.Stdout, opts)
	default:
		h = slog.NewTextHandler(os.Stdout, opts)
	}
	slog.SetDefault(slog.New(h))
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Info logs at Info level (key-value args after msg, slog-style).
func Info(msg string, args ...any) {
	slog.Default().Info(msg, args...)
}

// Warn logs at Warn level.
func Warn(msg string, args ...any) {
	slog.Default().Warn(msg, args...)
}

// Error logs at Error level.
func Error(msg string, args ...any) {
	slog.Default().Error(msg, args...)
}

// Debug logs at Debug level.
func Debug(msg string, args ...any) {
	slog.Default().Debug(msg, args...)
}

// Fatal logs at Error level and exits with code 1.
func Fatal(msg string, args ...any) {
	slog.Default().Error(msg, args...)
	os.Exit(1)
}
