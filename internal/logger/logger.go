// Package logger provides structured JSON logging.
package logger

import (
	"io"
	"log/slog"
)

// New creates a structured JSON logger with the specified log level.
// It writes JSON-formatted log entries to os.Stdout.
func New(level string) (*slog.Logger, error) {
	// TODO: implement in task group 4
	return slog.Default(), nil
}

// NewWithWriter creates a structured JSON logger that writes to the given writer.
// This is primarily used for testing to capture log output.
func NewWithWriter(level string, w io.Writer) (*slog.Logger, error) {
	// TODO: implement in task group 4
	return slog.Default(), nil
}
