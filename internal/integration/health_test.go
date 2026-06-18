package integration_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestTS01_6_HealthzReturns200 verifies that GET /healthz returns 200 OK when
// the service process is running.
// Requirement: 01-REQ-3.1 | Test Spec: TS-01-6
func TestTS01_6_HealthzReturns200(t *testing.T) {
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

// TestTS01_7_ReadyzReturns200WhenDBAvailable verifies that GET /readyz returns
// 200 OK when the SELECT 1 query against the database succeeds.
// Requirement: 01-REQ-3.2 | Test Spec: TS-01-7
func TestTS01_7_ReadyzReturns200WhenDBAvailable(t *testing.T) {
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

// TestTS01_E8_ReadyzReturns503WhenDBUnavailable verifies that GET /readyz
// returns 503 Service Unavailable with body {"message": "service unavailable"}
// when the database connection is unavailable or the SELECT 1 query fails.
// Requirement: 01-REQ-3.E1 | Test Spec: TS-01-E8
func TestTS01_E8_ReadyzReturns503WhenDBUnavailable(t *testing.T) {
	app := setupTestAppWithBrokenDB(t, testBearerToken)
	defer app.teardown()

	resp, err := http.Get(app.Server.URL + "/readyz")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", resp.StatusCode)
	}

	// Verify response Content-Type is application/json.
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	// Verify response body is exactly {"message": "service unavailable"}.
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
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
