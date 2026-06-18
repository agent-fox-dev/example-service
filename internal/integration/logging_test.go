package integration_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/agent-fox/example-service/internal/handler"
	"github.com/agent-fox/example-service/internal/logger"
)

// TestTS01_11_LogOutputIsJSON verifies that all log entries emitted by the
// service are formatted as valid JSON objects.
// Requirement: 01-REQ-5.1 | Test Spec: TS-01-11
func TestTS01_11_LogOutputIsJSON(t *testing.T) {
	// Create a logger that writes to a buffer so we can capture output.
	var logBuf bytes.Buffer

	slogger, err := logger.NewWithWriter("info", &logBuf)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	livenessHandler := &handler.LivenessHandler{}
	e.GET("/healthz", livenessHandler.Handle)

	server := httptest.NewServer(e)
	defer server.Close()

	// Make a request to generate log output.
	resp, err := http.Get(server.URL + "/healthz")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Use the logger to emit a test log entry. When the logger is properly
	// implemented, this should produce a JSON line in logBuf.
	slogger.Info("test log entry", slog.String("endpoint", "/healthz"))

	logOutput := logBuf.String()
	if logOutput == "" {
		t.Fatal("expected log output to be non-empty; logger should produce JSON output")
	}

	for _, line := range strings.Split(strings.TrimSpace(logOutput), "\n") {
		if line == "" {
			continue
		}
		var jsonObj map[string]interface{}
		if err := json.Unmarshal([]byte(line), &jsonObj); err != nil {
			t.Errorf("log line is not valid JSON: %q, error: %v", line, err)
		}
	}
}

// TestTS01_12_LogLevelFiltering verifies that the service honours the
// LOG_LEVEL environment variable to filter log output at runtime, suppressing
// messages below the configured level.
// Requirement: 01-REQ-5.2 | Test Spec: TS-01-12
func TestTS01_12_LogLevelFiltering(t *testing.T) {
	// Create a logger with LOG_LEVEL=warn that writes to a buffer.
	var logBuf bytes.Buffer

	slogger, err := logger.NewWithWriter("warn", &logBuf)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	livenessHandler := &handler.LivenessHandler{}
	e.GET("/healthz", livenessHandler.Handle)

	server := httptest.NewServer(e)
	defer server.Close()

	// Make a request.
	resp, err := http.Get(server.URL + "/healthz")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	// Emit log entries at various levels. Only warn and error should appear.
	slogger.Debug("debug message")
	slogger.Info("info message")
	slogger.Warn("warn message")
	slogger.Error("error message")

	logOutput := logBuf.String()
	if logOutput == "" {
		t.Fatal("expected log output to be non-empty; logger should produce JSON output for warn/error levels")
	}

	for _, line := range strings.Split(strings.TrimSpace(logOutput), "\n") {
		if line == "" {
			continue
		}
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Errorf("log line is not valid JSON: %q", line)
			continue
		}

		level, _ := entry["level"].(string)
		level = strings.ToLower(level)
		if level == "debug" || level == "info" {
			t.Errorf("found log entry with level %q when LOG_LEVEL=warn; entry: %s", level, line)
		}
	}
}
