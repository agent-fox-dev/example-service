// Package logger provides structured JSON logging.
package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// parseLevel maps a log level string to the corresponding slog.Level.
func parseLevel(level string) (slog.Level, error) {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("invalid log level: %q", level)
	}
}

// New creates a structured JSON logger with the specified log level.
// It writes JSON-formatted log entries to os.Stdout.
func New(level string) (*slog.Logger, error) {
	return NewWithWriter(level, os.Stdout)
}

// NewWithWriter creates a structured JSON logger that writes to the given writer.
// This is primarily used for testing to capture log output.
func NewWithWriter(level string, w io.Writer) (*slog.Logger, error) {
	lvl, err := parseLevel(level)
	if err != nil {
		return nil, err
	}

	h := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: lvl,
	})

	return slog.New(h), nil
}
