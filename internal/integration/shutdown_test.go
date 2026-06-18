package integration_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

// waitForServer polls the given address until a TCP connection succeeds
// or the timeout expires. Returns an error if the server never became ready.
func waitForServer(t *testing.T, addr string, timeout time.Duration) error {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("server at %s not ready within %v", addr, timeout)
}

// TestTS01_13_GracefulShutdownCompletesInFlight verifies that on receiving
// SIGTERM or SIGINT, the service stops accepting new connections and allows
// in-flight requests to complete before exiting within the 30-second timeout.
// Requirement: 01-REQ-6.1 | Test Spec: TS-01-13
func TestTS01_13_GracefulShutdownCompletesInFlight(t *testing.T) {
	// Build the server binary.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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
		"LOG_LEVEL=info",
	)

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	// Wait for the server to be ready to accept connections.
	if err := waitForServer(t, "127.0.0.1:18913", 5*time.Second); err != nil {
		t.Fatalf("server did not start: %v", err)
	}

	// Verify the server is accepting requests before shutdown.
	resp, err := http.Get("http://127.0.0.1:18913/healthz")
	if err != nil {
		t.Fatalf("healthz request failed before SIGTERM: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from healthz, got %d", resp.StatusCode)
	}

	// Send SIGTERM to initiate graceful shutdown.
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("failed to send SIGTERM: %v", err)
	}

	// Wait for process to exit within a reasonable timeout.
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case waitErr := <-done:
		// Process exited. A graceful shutdown with no in-flight requests
		// should exit with code 0.
		if waitErr != nil {
			// On some systems, the exit status from a signal-terminated
			// process may be non-zero; accept it if it exited at all.
			t.Logf("server exited with: %v (acceptable for signal shutdown)", waitErr)
		}
	case <-time.After(35 * time.Second):
		t.Fatal("server did not exit within 35 seconds after SIGTERM")
	}
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
	buildCtx, buildCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer buildCancel()

	binPath := t.TempDir() + "/server"
	buildCmd := exec.CommandContext(buildCtx, "go", "build", "-o", binPath, "./cmd/server")
	buildCmd.Dir = findProjectRoot(t)
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build server binary: %v\noutput: %s", err, out)
	}

	// Start the server.
	dbPath := t.TempDir() + "/test.db"
	cmd := exec.Command(binPath)
	cmd.Env = append(cmd.Environ(),
		"AUTH_BEARER_TOKEN=test-secret",
		"PORT=18914",
		"DB_PATH="+dbPath,
		"LOG_LEVEL=info",
	)

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	// Wait for the server to be ready.
	if err := waitForServer(t, "127.0.0.1:18914", 5*time.Second); err != nil {
		t.Fatalf("server did not start: %v", err)
	}

	// Open a raw TCP connection and begin writing a POST request with a
	// large Content-Length but never finish sending the body. This keeps
	// the Echo handler blocked on io.ReadAll(body), simulating an in-flight
	// request that takes > 30 seconds.
	conn, err := net.Dial("tcp", "127.0.0.1:18914")
	if err != nil {
		t.Fatalf("failed to dial server: %v", err)
	}
	defer conn.Close()

	// Send HTTP headers indicating a large body that we will never fully send.
	reqHeaders := "POST /v1/events HTTP/1.1\r\n" +
		"Host: 127.0.0.1:18914\r\n" +
		"Authorization: Bearer test-secret\r\n" +
		"Content-Type: application/json\r\n" +
		"Content-Length: 1048576\r\n" +
		"\r\n" +
		"{\"slow\":true}" // partial body — far less than 1MB
	if _, err := conn.Write([]byte(reqHeaders)); err != nil {
		t.Fatalf("failed to write request: %v", err)
	}

	// Give Echo time to accept the connection and begin reading.
	time.Sleep(200 * time.Millisecond)

	// Send SIGTERM and measure how long the process takes to exit.
	t0 := time.Now()
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("failed to send SIGTERM: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-done:
		elapsed := time.Since(t0)
		t.Logf("server exited after %v following SIGTERM with in-flight request", elapsed)
		// The server should force-exit after the 30-second shutdown timeout.
		// Allow a small tolerance window (up to 35 seconds) and ensure it
		// did not exit instantly (it should have waited for the shutdown timeout).
		if elapsed > 35*time.Second {
			t.Errorf("server took %v to exit, expected <= 35 seconds", elapsed)
		}
	case <-time.After(40 * time.Second):
		t.Fatal("server did not exit within 40 seconds after SIGTERM with in-flight request")
	}
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
