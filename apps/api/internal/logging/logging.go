// Package logging builds the API's structured logger. All request/response
// logging goes through this so log lines are consistently JSON and never
// carry request bodies, headers, or anything that could be a secret.
package logging

import (
	"log/slog"
	"os"

	"github.com/HealthCarefund/carefund-platform/apps/api/internal/config"
)

// New builds a JSON slog.Logger writing to stderr at the configured level.
func New(level config.LogLevel) *slog.Logger {
	var lvl slog.Level
	switch level {
	case config.LogLevelDebug:
		lvl = slog.LevelDebug
	case config.LogLevelWarn:
		lvl = slog.LevelWarn
	case config.LogLevelError:
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})
	return slog.New(handler)
}
