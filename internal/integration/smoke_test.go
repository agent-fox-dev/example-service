package integration_test

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestTS01_SMOKE1_EndToEndEventIngestion is a full end-to-end smoke test of
// successful audit event ingestion: a valid POST /v1/events request flows
// through auth, validation, SQLite persistence, and returns 201 Created.
// Updated for spec 02: uses canonical AuditEvent; id is the event's own id,
// not a server-generated UUID.
// Execution Path: 01-PATH-1
func TestTS01_SMOKE1_EndToEndEventIngestion(t *testing.T) {
	app := setupTestApp(t, testBearerToken)
	defer app.teardown()

	eventID := "ts01-smoke1-event-id"
	body := canonicalAuditEventWithID(eventID)
	req, err := http.NewRequest(http.MethodPost, app.Server.URL+"/v1/events", strings.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Assert HTTP 201 Created response with empty body.
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status 201, got %d", resp.StatusCode)
	}
	respBody, _ := io.ReadAll(resp.Body)
	if len(respBody) != 0 {
		t.Errorf("expected empty response body, got %d bytes", len(respBody))
	}

	// Assert no Content-Type header on the response.
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		t.Errorf("expected no Content-Type header, got %q", ct)
	}

	// Assert one new row in the events table with the event's own id
	// and received_at set to current UTC time.
	row := app.queryEventByID(t, eventID)
	if row.ID != eventID {
		t.Errorf("expected id %q, got %q", eventID, row.ID)
	}

	// Verify payload is semantically correct JSON.
	var payloadData map[string]any
	if err := json.Unmarshal([]byte(row.Payload), &payloadData); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if payloadData["plan_hash"] != "abc123" {
		t.Errorf("expected payload plan_hash %q, got %v", "abc123", payloadData["plan_hash"])
	}

	parsedTime, err := time.Parse(time.RFC3339Nano, row.ReceivedAt)
	if err != nil {
		t.Fatalf("failed to parse received_at %q: %v", row.ReceivedAt, err)
	}
	if time.Since(parsedTime) > 10*time.Second {
		t.Errorf("received_at %v is too far from current time", parsedTime)
	}
}

// TestTS01_SMOKE2_MissingAuthReturns401 is a smoke test for rejection of a
// POST /v1/events request with a missing Authorization header, verifying the
// auth middleware returns 401 and nothing is written to the database.
// Execution Path: 01-PATH-2
func TestTS01_SMOKE2_MissingAuthReturns401(t *testing.T) {
	app := setupTestApp(t, testBearerToken)
	defer app.teardown()

	body := `{"type":"smoke"}`
	req, err := http.NewRequest(http.MethodPost, app.Server.URL+"/v1/events", strings.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// No Authorization header.

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Assert HTTP 401 Unauthorized.
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", resp.StatusCode)
	}

	// Assert response body is {"message":"..."} with Content-Type: application/json.
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
	respBody, _ := io.ReadAll(resp.Body)
	var errResp map[string]any
	if err := json.Unmarshal(respBody, &errResp); err != nil {
		t.Fatalf("response body is not valid JSON: %v", err)
	}
	if _, ok := errResp["message"]; !ok {
		t.Error("response body does not contain 'message' field")
	}

	// Assert no rows inserted into the events table.
	count := app.eventRowCount(t)
	if count != 0 {
		t.Errorf("expected 0 rows in events table, got %d", count)
	}
}

// TestTS01_SMOKE3_MissingContentTypeReturns400 is a smoke test for rejection
// of a POST /v1/events request with a valid token but missing Content-Type
// header, verifying 400 Bad Request is returned and nothing is written to the
// database.
// Execution Path: 01-PATH-3
func TestTS01_SMOKE3_MissingContentTypeReturns400(t *testing.T) {
	app := setupTestApp(t, testBearerToken)
	defer app.teardown()

	body := `{"type":"smoke"}`
	req, err := http.NewRequest(http.MethodPost, app.Server.URL+"/v1/events", strings.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	req.Header.Del("Content-Type")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Assert HTTP 400 Bad Request.
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", resp.StatusCode)
	}

	// Assert response body is {"message":"..."} with Content-Type: application/json.
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
	respBody, _ := io.ReadAll(resp.Body)
	var errResp map[string]any
	if err := json.Unmarshal(respBody, &errResp); err != nil {
		t.Fatalf("response body is not valid JSON: %v", err)
	}
	if _, ok := errResp["message"]; !ok {
		t.Error("response body does not contain 'message' field")
	}

	// Assert no rows inserted into the events table.
	count := app.eventRowCount(t)
	if count != 0 {
		t.Errorf("expected 0 rows in events table, got %d", count)
	}
}

// TestTS01_SMOKE4_HealthzReturns200 is a smoke test for the liveness probe:
// GET /healthz returns 200 OK immediately without database interaction.
// Execution Path: 01-PATH-4
func TestTS01_SMOKE4_HealthzReturns200(t *testing.T) {
	app := setupTestApp(t, testBearerToken)
	defer app.teardown()

	resp, err := http.Get(app.Server.URL + "/healthz")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

// TestTS01_SMOKE5_ReadyzDBAvailable is a smoke test for the readiness probe
// when the database is available: GET /readyz executes SELECT 1 successfully
// and returns 200 OK.
// Execution Path: 01-PATH-5
func TestTS01_SMOKE5_ReadyzDBAvailable(t *testing.T) {
	app := setupTestApp(t, testBearerToken)
	defer app.teardown()

	resp, err := http.Get(app.Server.URL + "/readyz")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

// TestTS01_SMOKE6_ReadyzDBUnavailable is a smoke test for the readiness probe
// when the database is unavailable: GET /readyz returns 503 Service
// Unavailable with the expected error body.
// Execution Path: 01-PATH-6
func TestTS01_SMOKE6_ReadyzDBUnavailable(t *testing.T) {
	app := setupTestAppWithBrokenDB(t, testBearerToken)
	defer app.teardown()

	resp, err := http.Get(app.Server.URL + "/readyz")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Assert HTTP 503 Service Unavailable.
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", resp.StatusCode)
	}

	// Assert response body is {"message":"service unavailable"} with
	// Content-Type: application/json.
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
	respBody, _ := io.ReadAll(resp.Body)
	var errResp map[string]any
	if err := json.Unmarshal(respBody, &errResp); err != nil {
		t.Fatalf("response body is not valid JSON: %v", err)
	}
	msg, ok := errResp["message"]
	if !ok {
		t.Error("response body does not contain 'message' field")
	} else if msg != "service unavailable" {
		t.Errorf("expected message 'service unavailable', got %q", msg)
	}
}

// TestTS01_SMOKE7_GracefulShutdown is a smoke test for graceful shutdown:
// sending SIGTERM causes the service to stop accepting new connections,
// complete in-flight requests, and exit within 30 seconds.
// Execution Path: 01-PATH-7
func TestTS01_SMOKE7_GracefulShutdown(t *testing.T) {
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
		"PORT=18915",
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
	if err := waitForServer(t, "127.0.0.1:18915", 5*time.Second); err != nil {
		t.Fatalf("server did not start: %v", err)
	}

	// Send a POST /v1/events request to verify the full pipeline works
	// before sending SIGTERM. Uses canonical AuditEvent per spec 02.
	body := canonicalAuditEventWithID("ts01-smoke7-shutdown-id")
	req, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:18915/v1/events", strings.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer test-secret")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("pre-shutdown POST request failed: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 from pre-shutdown POST, got %d", resp.StatusCode)
	}

	// Send SIGTERM.
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("failed to send SIGTERM: %v", err)
	}

	// Wait for process to exit within 35 seconds.
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case waitErr := <-done:
		if waitErr != nil {
			t.Logf("server exited with: %v (acceptable for signal shutdown)", waitErr)
		}
	case <-time.After(35 * time.Second):
		t.Fatal("server did not exit within 35 seconds after SIGTERM")
	}

	// Verify that new connections are refused after shutdown.
	_, err = net.DialTimeout("tcp", "127.0.0.1:18915", 500*time.Millisecond)
	if err == nil {
		t.Error("expected connection to be refused after shutdown, but it succeeded")
	}
}
