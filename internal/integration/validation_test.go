package integration_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestTS01_E1_MissingContentTypeHeader verifies that POST /v1/events with a
// valid bearer token but missing Content-Type header returns 400 Bad Request
// with an Echo Framework error body and does not write to the events table.
// Requirement: 01-REQ-1.E1 | Test Spec: TS-01-E1
func TestTS01_E1_MissingContentTypeHeader(t *testing.T) {
	app := setupTestApp(t, testBearerToken)
	defer app.teardown()

	body := `{"event":"test"}`
	req, err := http.NewRequest(http.MethodPost, app.Server.URL+"/v1/events", strings.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	// Deliberately not setting Content-Type header.
	// Clear any default Content-Type that may be set.
	req.Header.Del("Content-Type")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", resp.StatusCode)
	}

	// Verify response Content-Type is application/json.
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	// Verify response body contains a "message" field.
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	var errResp map[string]any
	if err := json.Unmarshal(respBody, &errResp); err != nil {
		t.Fatalf("response body is not valid JSON: %v", err)
	}
	if _, ok := errResp["message"]; !ok {
		t.Error("response body does not contain 'message' field")
	}

	// Verify no rows were inserted.
	count := app.eventRowCount(t)
	if count != 0 {
		t.Errorf("expected 0 rows in events table, got %d", count)
	}
}

// TestTS01_E2_NonJSONContentType verifies that POST /v1/events with
// Content-Type set to a non-JSON value returns 400 Bad Request and does not
// write to the events table.
// Requirement: 01-REQ-1.E1 | Test Spec: TS-01-E2
func TestTS01_E2_NonJSONContentType(t *testing.T) {
	app := setupTestApp(t, testBearerToken)
	defer app.teardown()

	body := `{"event":"test"}`
	req, err := http.NewRequest(http.MethodPost, app.Server.URL+"/v1/events", strings.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	req.Header.Set("Content-Type", "text/plain")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", resp.StatusCode)
	}

	// Verify response Content-Type is application/json.
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	// Verify response body contains a "message" field.
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	var errResp map[string]any
	if err := json.Unmarshal(respBody, &errResp); err != nil {
		t.Fatalf("response body is not valid JSON: %v", err)
	}
	if _, ok := errResp["message"]; !ok {
		t.Error("response body does not contain 'message' field")
	}

	// Verify no rows were inserted.
	count := app.eventRowCount(t)
	if count != 0 {
		t.Errorf("expected 0 rows in events table, got %d", count)
	}
}

// TestTS01_E3_EmptyRequestBody verifies that POST /v1/events with a valid
// bearer token and Content-Type but an empty request body returns 400 Bad
// Request and does not write to the events table.
// Requirement: 01-REQ-1.E2 | Test Spec: TS-01-E3
func TestTS01_E3_EmptyRequestBody(t *testing.T) {
	app := setupTestApp(t, testBearerToken)
	defer app.teardown()

	req, err := http.NewRequest(http.MethodPost, app.Server.URL+"/v1/events", strings.NewReader(""))
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

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", resp.StatusCode)
	}

	// Verify response Content-Type is application/json.
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	// Verify response body contains a "message" field.
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	var errResp map[string]any
	if err := json.Unmarshal(respBody, &errResp); err != nil {
		t.Fatalf("response body is not valid JSON: %v", err)
	}
	if _, ok := errResp["message"]; !ok {
		t.Error("response body does not contain 'message' field")
	}

	// Verify no rows were inserted.
	count := app.eventRowCount(t)
	if count != 0 {
		t.Errorf("expected 0 rows in events table, got %d", count)
	}
}

// TestTS01_E4_DatabaseInsertFailure verifies that when a database error occurs
// during event storage, the service returns 500 Internal Server Error with an
// Echo Framework error body.
// Requirement: 01-REQ-1.E3 | Test Spec: TS-01-E4
func TestTS01_E4_DatabaseInsertFailure(t *testing.T) {
	app := setupTestApp(t, testBearerToken)
	defer app.teardown()

	// Drop the events table to simulate a database insert failure.
	_, err := app.DB.Exec("DROP TABLE events")
	if err != nil {
		t.Fatalf("failed to drop events table: %v", err)
	}

	body := `{"event":"test"}`
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

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", resp.StatusCode)
	}

	// Verify response Content-Type is application/json.
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	// Verify response body contains a "message" field.
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	var errResp map[string]any
	if err := json.Unmarshal(respBody, &errResp); err != nil {
		t.Fatalf("response body is not valid JSON: %v", err)
	}
	if _, ok := errResp["message"]; !ok {
		t.Error("response body does not contain 'message' field")
	}
}
