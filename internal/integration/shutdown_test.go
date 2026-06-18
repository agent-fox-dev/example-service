package integration_test

import (
	"context"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestTS01_13_GracefulShutdownCompletesInFlight verifies that on receiving
// SIGTERM or SIGINT, the service stops accepting new connections and allows
// in-flight requests to complete before exiting within the 30-second timeout.
// Requirement: 01-REQ-6.1 | Test Spec: TS-01-13
func TestTS01_13_GracefulShutdownCompletesInFlight(t *testing.T) {
	// Build the server binary.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	binPath := t.TempDir() + "/server"
	buildCmd := exec.CommandContext(ctx, "go", "build", "-o", binPath, "./cmd/server")
	buildCmd.Dir = findProjectRoot(t)
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build server binary: %v\noutput: %s", err, out)
	}

	// Start the server with a test token.
	dbPath := t.TempDir() + "/test.db"
	cmd := exec.Command(binPath)
	cmd.Env = append(cmd.Environ(),
		"AUTH_BEARER_TOKEN=test-secret",
		"PORT=18913",
		"DB_PATH="+dbPath,
	)

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	// Give the server time to start.
	time.Sleep(500 * time.Millisecond)

	// Send SIGTERM to initiate graceful shutdown.
	if cmd.Process != nil {
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			t.Logf("failed to send SIGTERM: %v", err)
		}
	}

	// Wait for process to exit within a reasonable timeout.
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-done:
		// Process exited — when fully implemented, we'd also verify:
		// 1. In-flight request completed with 201
		// 2. New connections were refused after SIGTERM
	case <-time.After(35 * time.Second):
		t.Fatal("server did not exit within 35 seconds after SIGTERM")
	}

	// The stub server exits immediately because main() is empty.
	// When fully implemented with signal handling, this test should verify
	// that in-flight requests complete before the server exits.
	t.Error("graceful shutdown test requires full server implementation with signal handling")
}

// TestTS01_14_ForceExitAfterTimeout verifies that the service force-exits
// after 30 seconds if in-flight requests have not completed during graceful
// shutdown.
// Requirement: 01-REQ-6.2 | Test Spec: TS-01-14
func TestTS01_14_ForceExitAfterTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow shutdown timeout test in short mode")
	}

	// Build the server binary.
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	binPath := t.TempDir() + "/server"
	buildCmd := exec.CommandContext(ctx, "go", "build", "-o", binPath, "./cmd/server")
	buildCmd.Dir = findProjectRoot(t)
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build server binary: %v\noutput: %s", err, out)
	}

	// When fully implemented, this test should:
	// 1. Start the server
	// 2. Begin a request handler that blocks for 60 seconds
	// 3. Send SIGTERM
	// 4. Verify the process exits within ~32 seconds (30s timeout + tolerance)
	// 5. Verify (t1 - t0) <= 32 seconds

	t.Error("force exit timeout test requires full server implementation with slow handler support")

	_ = binPath
}

// TestTS01_15_IntegrationTestSuiteIncludesIngestion is a meta-test that
// verifies the integration test suite includes a test for successful event
// ingestion that asserts 201 Created and verifies the event is persisted
// in SQLite.
// Requirement: 01-REQ-7.1 | Test Spec: TS-01-15
func TestTS01_15_IntegrationTestSuiteIncludesIngestion(t *testing.T) {
	verifyTestExists(t, "TestTS01_1_SuccessfulEventIngestion",
		"integration test for successful event ingestion (201 + DB row)")
}

// TestTS01_16_IntegrationTestSuiteIncludesAuth is a meta-test that verifies
// the integration test suite includes tests for missing and invalid bearer
// tokens that assert 401 Unauthorized responses.
// Requirement: 01-REQ-7.2 | Test Spec: TS-01-16
func TestTS01_16_IntegrationTestSuiteIncludesAuth(t *testing.T) {
	verifyTestExists(t, "TestTS01_E5_MissingAuthorizationHeader",
		"integration test for missing authorization header (401)")
	verifyTestExists(t, "TestTS01_E6_MalformedAuthorizationHeader",
		"integration test for malformed authorization header (401)")
	verifyTestExists(t, "TestTS01_E7_WrongTokenValue",
		"integration test for wrong token value (401)")
}

// TestTS01_17_IntegrationTestSuiteIncludesPayloadValidation is a meta-test
// that verifies the integration test suite includes tests for missing
// Content-Type and empty body that assert 400 Bad Request responses.
// Requirement: 01-REQ-7.3 | Test Spec: TS-01-17
func TestTS01_17_IntegrationTestSuiteIncludesPayloadValidation(t *testing.T) {
	verifyTestExists(t, "TestTS01_E1_MissingContentTypeHeader",
		"integration test for missing Content-Type (400)")
	verifyTestExists(t, "TestTS01_E3_EmptyRequestBody",
		"integration test for empty body (400)")
}

// TestTS01_18_IntegrationTestSuiteIncludesHealthChecks is a meta-test that
// verifies the integration test suite includes tests for GET /healthz and
// GET /readyz under normal conditions that assert 200 OK responses.
// Requirement: 01-REQ-7.4 | Test Spec: TS-01-18
func TestTS01_18_IntegrationTestSuiteIncludesHealthChecks(t *testing.T) {
	verifyTestExists(t, "TestTS01_6_HealthzReturns200",
		"integration test for GET /healthz (200)")
	verifyTestExists(t, "TestTS01_7_ReadyzReturns200WhenDBAvailable",
		"integration test for GET /readyz (200)")
}

// verifyTestExists checks that a test with the given name exists in the
// integration test suite by running `go test -list`.
func verifyTestExists(t *testing.T, testName, description string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "test", "./internal/integration/...",
		"-list", "^"+testName+"$", "-count=1")
	cmd.Dir = findProjectRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("failed to list test %s: %v\noutput: %s", testName, err, out)
		return
	}

	if !strings.Contains(string(out), testName) {
		t.Errorf("test %s (%s) not found in integration test suite", testName, description)
	}
}

// findProjectRoot returns the directory containing go.mod.
func findProjectRoot(t *testing.T) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "env", "GOMOD")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to find project root: %v", err)
	}

	modPath := strings.TrimSpace(string(out))
	if modPath == "" {
		t.Fatal("could not determine project root from go env GOMOD")
	}

	// Return the directory containing go.mod.
	idx := strings.LastIndex(modPath, "/")
	if idx < 0 {
		t.Fatalf("unexpected go.mod path: %s", modPath)
	}
	return modPath[:idx]
}
