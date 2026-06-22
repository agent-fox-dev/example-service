package integration_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestTS01_1_SuccessfulEventIngestion verifies that a POST /v1/events request
// with a valid bearer token, Content-Type: application/json, and a non-empty
// body returns 201 Created with an empty response body and no Content-Type
// header, and stores the event in SQLite.
// Requirement: 01-REQ-1.1 | Test Spec: TS-01-1
func TestTS01_1_SuccessfulEventIngestion(t *testing.T) {
	app := setupTestApp(t, testBearerToken)
	defer app.teardown()

	body := canonicalAuditEventWithID("ts01-1-event-id")
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

	// Assert HTTP 201 Created.
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status 201, got %d", resp.StatusCode)
	}

	// Assert empty response body.
	respBody := make([]byte, 1)
	n, _ := resp.Body.Read(respBody)
	if n != 0 {
		t.Errorf("expected empty response body, got %d bytes", n)
	}

	// Assert no Content-Type header on the response.
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		t.Errorf("expected no Content-Type header, got %q", ct)
	}

	// Assert one row in the events table.
	count := app.eventRowCount(t)
	if count != 1 {
		t.Errorf("expected 1 row in events table, got %d", count)
	}
}

// TestTS01_2_RawJSONStoredVerbatim verifies that the service stores the
// event payload as re-serialized JSON (semantically equivalent to submitted).
// Updated for spec 02: uses canonical AuditEvent payload.
// Requirement: 01-REQ-1.2 | Test Spec: TS-01-2
func TestTS01_2_RawJSONStoredVerbatim(t *testing.T) {
	app := setupTestApp(t, testBearerToken)
	defer app.teardown()

	body := canonicalAuditEventWithID("ts01-2-event-id")
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

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status 201, got %d", resp.StatusCode)
	}

	var payload string
	err = app.DB.QueryRow("SELECT payload FROM events WHERE id = ?", "ts01-2-event-id").Scan(&payload)
	if err != nil {
		t.Fatalf("failed to query event payload: %v", err)
	}

	// Use semantic JSON comparison since payload is re-serialized via
	// json.Marshal and byte-for-byte equality is not guaranteed (02-REQ-4.3).
	var storedPayload, expectedPayload map[string]any
	if err := json.Unmarshal([]byte(payload), &storedPayload); err != nil {
		t.Fatalf("stored payload is not valid JSON: %v", err)
	}
	if err := json.Unmarshal([]byte(`{"plan_hash":"abc123"}`), &expectedPayload); err != nil {
		t.Fatalf("expected payload is not valid JSON: %v", err)
	}
	if storedPayload["plan_hash"] != expectedPayload["plan_hash"] {
		t.Errorf("payload mismatch:\nstored: %s\nexpected plan_hash: %s", payload, `abc123`)
	}
}

// TestTS01_3_EventIDAndReceivedAtGenerated verifies that the event's own
// id field is used as the primary key and received_at is set to the current
// UTC time on every insert into the events table.
// Updated for spec 02: uses canonical AuditEvent; id is the event's own id,
// not a server-generated UUID (02-REQ-4.2).
// Requirement: 01-REQ-1.3 | Test Spec: TS-01-3
func TestTS01_3_EventIDAndReceivedAtGenerated(t *testing.T) {
	app := setupTestApp(t, testBearerToken)
	defer app.teardown()

	before := time.Now().UTC().Add(-2 * time.Second)

	eventID := "ts01-3-event-id"
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

	after := time.Now().UTC().Add(2 * time.Second)

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status 201, got %d", resp.StatusCode)
	}

	var id, receivedAt string
	err = app.DB.QueryRow("SELECT id, received_at FROM events WHERE id = ?", eventID).Scan(&id, &receivedAt)
	if err != nil {
		t.Fatalf("failed to query event row: %v", err)
	}

	// Verify id matches the submitted event's own id (not a server-side UUID).
	if id != eventID {
		t.Errorf("expected id %q, got %q", eventID, id)
	}

	// Verify received_at is within the expected time window.
	parsedTime, err := time.Parse(time.RFC3339Nano, receivedAt)
	if err != nil {
		t.Fatalf("failed to parse received_at %q: %v", receivedAt, err)
	}

	if parsedTime.Before(before) || parsedTime.After(after) {
		t.Errorf("received_at %v is not within expected range [%v, %v]", parsedTime, before, after)
	}
}

// TestTS01_4_BearerTokenExtractionFormat verifies that the service extracts
// the bearer token using exactly the 'Bearer <token>' format (capital B,
// single space) and compares it against AUTH_BEARER_TOKEN.
// Requirement: 01-REQ-2.1 | Test Spec: TS-01-4
func TestTS01_4_BearerTokenExtractionFormat(t *testing.T) {
	token := "my-secret-token"
	app := setupTestApp(t, token)
	defer app.teardown()

	body := `{"e":1}`
	req, err := http.NewRequest(http.MethodPost, app.Server.URL+"/v1/events", strings.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status 201 (token extracted and matched correctly), got %d", resp.StatusCode)
	}
}

// TestTS01_5_ValidTokenProceedsToStorage verifies that when the extracted
// token matches AUTH_BEARER_TOKEN, the request proceeds to payload validation
// and storage.
// Requirement: 01-REQ-2.2 | Test Spec: TS-01-5
func TestTS01_5_ValidTokenProceedsToStorage(t *testing.T) {
	token := "valid-token"
	app := setupTestApp(t, token)
	defer app.teardown()

	body := canonicalAuditEventWithID("ts01-5-event-id")
	req, err := http.NewRequest(http.MethodPost, app.Server.URL+"/v1/events", strings.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status 201, got %d", resp.StatusCode)
	}

	count := app.eventRowCount(t)
	if count != 1 {
		t.Errorf("expected 1 row in events table, got %d", count)
	}
}
