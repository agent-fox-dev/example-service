package integration_test

import (
	"strings"
	"testing"

	"github.com/agent-fox/example-service/internal/config"
)

// TestTS01_8_DefaultConfigValues verifies that PORT, DB_PATH,
// AUTH_BEARER_TOKEN, and LOG_LEVEL are read from environment variables at
// startup, with correct defaults applied when optional variables are absent.
// Requirement: 01-REQ-4.1 | Test Spec: TS-01-8
func TestTS01_8_DefaultConfigValues(t *testing.T) {
	// Set only the required AUTH_BEARER_TOKEN and unset optional vars.
	t.Setenv("AUTH_BEARER_TOKEN", "env-test-token")
	t.Setenv("PORT", "")
	t.Setenv("DB_PATH", "")
	t.Setenv("LOG_LEVEL", "")

	// Clear the env vars so Load() sees them as absent.
	// t.Setenv with empty string may not be equivalent to unset on all
	// platforms, so we use os.Unsetenv-like approach via empty string.
	// The Load() function should treat empty PORT, DB_PATH, LOG_LEVEL
	// as absent and apply defaults.

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() returned error: %v", err)
	}

	if cfg.Port != "8080" {
		t.Errorf("expected default Port '8080', got %q", cfg.Port)
	}

	if cfg.DBPath != "./data/events.db" {
		t.Errorf("expected default DBPath './data/events.db', got %q", cfg.DBPath)
	}

	if cfg.LogLevel != "info" {
		t.Errorf("expected default LogLevel 'info', got %q", cfg.LogLevel)
	}

	if cfg.AuthBearerToken != "env-test-token" {
		t.Errorf("expected AuthBearerToken 'env-test-token', got %q", cfg.AuthBearerToken)
	}
}

// TestTS01_9_MissingAuthBearerToken verifies that the service fails to start
// and emits a descriptive error message when AUTH_BEARER_TOKEN is absent from
// the environment.
// Requirement: 01-REQ-4.2 | Test Spec: TS-01-9
func TestTS01_9_MissingAuthBearerToken(t *testing.T) {
	// Ensure AUTH_BEARER_TOKEN is unset.
	t.Setenv("AUTH_BEARER_TOKEN", "")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected config.Load() to return an error when AUTH_BEARER_TOKEN is absent")
	}

	// Verify the error message references AUTH_BEARER_TOKEN.
	if !strings.Contains(err.Error(), "AUTH_BEARER_TOKEN") {
		t.Errorf("error message should reference AUTH_BEARER_TOKEN, got: %v", err)
	}
}

// TestTS01_10_InvalidLogLevel verifies that the service fails to start with a
// descriptive error message when LOG_LEVEL is set to an invalid value.
// Requirement: 01-REQ-4.3 | Test Spec: TS-01-10
func TestTS01_10_InvalidLogLevel(t *testing.T) {
	t.Setenv("AUTH_BEARER_TOKEN", "test-secret")
	t.Setenv("LOG_LEVEL", "verbose")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected config.Load() to return an error when LOG_LEVEL is invalid")
	}

	// Verify the error message references LOG_LEVEL or indicates invalid value.
	errMsg := err.Error()
	if !strings.Contains(errMsg, "LOG_LEVEL") && !strings.Contains(strings.ToLower(errMsg), "invalid") {
		t.Errorf("error message should reference LOG_LEVEL or 'invalid', got: %v", err)
	}
}
