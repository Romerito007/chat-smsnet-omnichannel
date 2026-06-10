package shared

import (
	"context"
	"log/slog"
	"os"
)

// Logger is the structured logger used across the backend. We alias slog so the
// domain depends on the standard library rather than a third-party logger.
type Logger = *slog.Logger

// NewLogger builds a JSON structured logger at the given level ("debug",
// "info", "warn", "error").
func NewLogger(level string) Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	return slog.New(handler)
}

// LoggerFrom returns a logger enriched with request_id and tenant_id pulled
// from the context, falling back to the default logger.
func LoggerFrom(ctx context.Context, base Logger) Logger {
	l := base
	if rid := RequestIDFrom(ctx); rid != "" {
		l = l.With("request_id", rid)
	}
	if t, ok := TenantFrom(ctx); ok {
		l = l.With("tenant_id", t)
	}
	return l
}
