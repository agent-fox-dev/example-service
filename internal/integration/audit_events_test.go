package integration_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// canonicalAuditEventID is the canonical event id used in spec 02 tests.
const canonicalAuditEventID = "7c10ec9b-daaf-4146-8e40-6efc92e5db39"

// canonicalAuditEvent returns the canonical valid AuditEvent JSON body
// used across spec 02 integration tests.
func canonicalAuditEvent() string {
	return `{"id":"7c10ec9b-daaf-4146-8e40-6efc92e5db39","timestamp":"2026-06-18T11:26:26.527713+00:00","run_id":"20260618_112626_17dd3f","event_type":"run.start","node_id":"","session_id":"","archetype":"","severity":"info","payload":{"plan_hash":"abc123"}}`
}

// canonicalAuditEventWithID returns a canonical valid AuditEvent JSON body
// with the given id value (for tests requiring distinct ids).
func canonicalAuditEventWithID(id string) string {
	return `{"id":"` + id + `","timestamp":"2026-06-18T11:26:26.527713+00:00","run_id":"20260618_112626_17dd3f","event_type":"run.start","node_id":"","session_id":"","archetype":"","severity":"info","payload":{"plan_hash":"abc123"}}`
}

// TestTS02_1_ValidAuditEventStoredCorrectly POSTs the canonical valid
// AuditEvent JSON, asserts HTTP 201 with a zero-byte body, queries the
// events table, and verifies all 9 event columns match the submitted values;
// asserts received_at is present and parseable as RFC3339Nano.
// Requirement: 02-REQ-9.1 | Test Spec: TS-02-20
func TestTS02_1_ValidAuditEventStoredCorrectly(t *testing.T) {
	app := setupTestApp(t, testBearerToken)
	defer app.teardown()

	body := canonicalAuditEvent()
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

	// Assert zero-byte body.
	respBody := make([]byte, 1)
	n, _ := resp.Body.Read(respBody)
	if n != 0 {
		t.Errorf("expected empty response body, got %d bytes", n)
	}

	// Assert no Content-Type header on the response.
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		t.Errorf("expected no Content-Type header, got %q", ct)
	}

	// Query the event row and verify all 9 event columns.
	row := app.queryEventByID(t, canonicalAuditEventID)

	if row.ID != "7c10ec9b-daaf-4146-8e40-6efc92e5db39" {
		t.Errorf("expected id %q, got %q", "7c10ec9b-daaf-4146-8e40-6efc92e5db39", row.ID)
	}
	if row.Timestamp != "2026-06-18T11:26:26.527713+00:00" {
		t.Errorf("expected timestamp %q, got %q", "2026-06-18T11:26:26.527713+00:00", row.Timestamp)
	}
	if row.RunID != "20260618_112626_17dd3f" {
		t.Errorf("expected run_id %q, got %q", "20260618_112626_17dd3f", row.RunID)
	}
	if row.EventType != "run.start" {
		t.Errorf("expected event_type %q, got %q", "run.start", row.EventType)
	}
	if row.NodeID != "" {
		t.Errorf("expected node_id %q, got %q", "", row.NodeID)
	}
	if row.SessionID != "" {
		t.Errorf("expected session_id %q, got %q", "", row.SessionID)
	}
	if row.Archetype != "" {
		t.Errorf("expected archetype %q, got %q", "", row.Archetype)
	}
	if row.Severity != "info" {
		t.Errorf("expected severity %q, got %q", "info", row.Severity)
	}

	// Verify payload is semantically equivalent JSON with plan_hash=abc123.
	var payloadData map[string]any
	if err := json.Unmarshal([]byte(row.Payload), &payloadData); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if payloadData["plan_hash"] != "abc123" {
		t.Errorf("expected payload plan_hash %q, got %v", "abc123", payloadData["plan_hash"])
	}

	// Verify received_at is present and parseable as RFC3339Nano.
	if row.ReceivedAt == "" {
		t.Fatal("received_at is empty")
	}
	parsedTime, err := time.Parse(time.RFC3339Nano, row.ReceivedAt)
	if err != nil {
		t.Fatalf("failed to parse received_at %q as RFC3339Nano: %v", row.ReceivedAt, err)
	}
	if parsedTime.Location() != time.UTC {
		t.Errorf("expected received_at to be UTC, got %v", parsedTime.Location())
	}
}

// TestTS02_2_InvalidPayloadsSilentlyRejected POSTs three invalid payloads
// as separate requests — (a) missing event_type, (b) payload as array,
// (c) empty id — asserts HTTP 201 for each and verifies no rows stored.
// Requirement: 02-REQ-9.2 | Test Spec: TS-02-21
func TestTS02_2_InvalidPayloadsSilentlyRejected(t *testing.T) {
	app := setupTestApp(t, testBearerToken)
	defer app.teardown()

	invalidBodies := []struct {
		name string
		body string
	}{
		{
			name: "missing_event_type",
			body: `{"id":"inv-1","timestamp":"t","run_id":"r","node_id":"","session_id":"","archetype":"","severity":"info","payload":{}}`,
		},
		{
			name: "array_payload",
			body: `{"id":"inv-2","timestamp":"t","run_id":"r","event_type":"e","node_id":"","session_id":"","archetype":"","severity":"info","payload":["not","an","object"]}`,
		},
		{
			name: "empty_id",
			body: `{"id":"","timestamp":"t","run_id":"r","event_type":"e","node_id":"","session_id":"","archetype":"","severity":"info","payload":{}}`,
		},
	}

	for _, tc := range invalidBodies {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, app.Server.URL+"/v1/events", strings.NewReader(tc.body))
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

			// Assert zero-byte body.
			respBody := make([]byte, 1)
			n, _ := resp.Body.Read(respBody)
			if n != 0 {
				t.Errorf("expected empty response body, got %d bytes", n)
			}
		})
	}

	// Verify no rows were stored across all three requests.
	count := app.eventRowCount(t)
	if count != 0 {
		t.Errorf("expected 0 rows in events table, got %d", count)
	}
}

// TestTS02_3_DuplicateEventIdempotent POSTs the canonical valid AuditEvent
// twice with the same id, asserts HTTP 201 both times, and verifies exactly
// one row exists in the events table.
// Requirement: 02-REQ-9.3 | Test Spec: TS-02-22
func TestTS02_3_DuplicateEventIdempotent(t *testing.T) {
	app := setupTestApp(t, testBearerToken)
	defer app.teardown()

	body := canonicalAuditEvent()

	for i := range 2 {
		req, err := http.NewRequest(http.MethodPost, app.Server.URL+"/v1/events", strings.NewReader(body))
		if err != nil {
			t.Fatalf("request %d: failed to create request: %v", i+1, err)
		}
		req.Header.Set("Authorization", "Bearer "+testBearerToken)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request %d: failed: %v", i+1, err)
		}
		defer func() { _ = resp.Body.Close() }()

		// Assert HTTP 201 Created for each request.
		if resp.StatusCode != http.StatusCreated {
			t.Errorf("request %d: expected status 201, got %d", i+1, resp.StatusCode)
		}

		// Assert zero-byte body.
		respBody := make([]byte, 1)
		n, _ := resp.Body.Read(respBody)
		if n != 0 {
			t.Errorf("request %d: expected empty response body, got %d bytes", i+1, n)
		}
	}

	// Verify exactly one row exists in the events table.
	count := app.eventRowCount(t)
	if count != 1 {
		t.Errorf("expected exactly 1 row in events table after duplicate submission, got %d", count)
	}
}

// --- Edge-case tests (TS-02-E*) ---

// TestTS02_E1_ExtraFieldsAccepted verifies that extra top-level fields beyond
// the 9 defined in AuditEvent are silently ignored and the event is stored.
// Requirement: 02-REQ-1.E1 | Test Spec: TS-02-E1
func TestTS02_E1_ExtraFieldsAccepted(t *testing.T) {
	app := setupTestApp(t, testBearerToken)
	defer app.teardown()

	body := `{"id":"extra-field-id","timestamp":"t","run_id":"r","event_type":"e","node_id":"","session_id":"","archetype":"","severity":"info","payload":{},"future_field":"value","another_extra":42}`
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

	respBody := make([]byte, 1)
	n, _ := resp.Body.Read(respBody)
	if n != 0 {
		t.Errorf("expected empty response body, got %d bytes", n)
	}

	count := app.eventRowCount(t)
	if count != 1 {
		t.Errorf("expected 1 row in events table (extra fields should be accepted), got %d", count)
	}
}

// TestTS02_E2_MalformedJSONRejected verifies that a non-empty body with
// malformed JSON is silently rejected with HTTP 201 and no row is stored.
// Requirement: 02-REQ-2.E1 | Test Spec: TS-02-E2
func TestTS02_E2_MalformedJSONRejected(t *testing.T) {
	app := setupTestApp(t, testBearerToken)
	defer app.teardown()

	body := "this is not json"
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

	respBody := make([]byte, 1)
	n, _ := resp.Body.Read(respBody)
	if n != 0 {
		t.Errorf("expected empty response body, got %d bytes", n)
	}

	if ct := resp.Header.Get("Content-Type"); ct != "" {
		t.Errorf("expected no Content-Type header, got %q", ct)
	}

	count := app.eventRowCount(t)
	if count != 0 {
		t.Errorf("expected 0 rows in events table, got %d", count)
	}
}

// TestTS02_E3_TopLevelArrayRejected verifies that a non-empty body that is
// a JSON array (not an object) is silently rejected with HTTP 201.
// Requirement: 02-REQ-2.E1 | Test Spec: TS-02-E3
func TestTS02_E3_TopLevelArrayRejected(t *testing.T) {
	app := setupTestApp(t, testBearerToken)
	defer app.teardown()

	body := `["not", "an", "object"]`
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

	respBody := make([]byte, 1)
	n, _ := resp.Body.Read(respBody)
	if n != 0 {
		t.Errorf("expected empty response body, got %d bytes", n)
	}

	count := app.eventRowCount(t)
	if count != 0 {
		t.Errorf("expected 0 rows in events table, got %d", count)
	}
}

// TestTS02_E4_TopLevelNullRejected verifies that a non-empty body that is
// a JSON null is silently rejected with HTTP 201.
// Requirement: 02-REQ-2.E1 | Test Spec: TS-02-E4
func TestTS02_E4_TopLevelNullRejected(t *testing.T) {
	app := setupTestApp(t, testBearerToken)
	defer app.teardown()

	body := "null"
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

	respBody := make([]byte, 1)
	n, _ := resp.Body.Read(respBody)
	if n != 0 {
		t.Errorf("expected empty response body, got %d bytes", n)
	}

	count := app.eventRowCount(t)
	if count != 0 {
		t.Errorf("expected 0 rows in events table, got %d", count)
	}
}

// TestTS02_E5_EmptyIDRejected verifies that an event with an empty string
// id is silently rejected with HTTP 201 and no row is stored.
// Requirement: 02-REQ-2.E2 | Test Spec: TS-02-E5
func TestTS02_E5_EmptyIDRejected(t *testing.T) {
	app := setupTestApp(t, testBearerToken)
	defer app.teardown()

	body := `{"id":"","timestamp":"t","run_id":"r","event_type":"e","node_id":"","session_id":"","archetype":"","severity":"info","payload":{}}`
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

	respBody := make([]byte, 1)
	n, _ := resp.Body.Read(respBody)
	if n != 0 {
		t.Errorf("expected empty response body, got %d bytes", n)
	}

	count := app.eventRowCount(t)
	if count != 0 {
		t.Errorf("expected 0 rows in events table, got %d", count)
	}
}

// TestTS02_E6_EmptySeverityRejected verifies that an event with an empty
// string severity is silently rejected with HTTP 201 and no row is stored.
// Requirement: 02-REQ-2.E2 | Test Spec: TS-02-E6
func TestTS02_E6_EmptySeverityRejected(t *testing.T) {
	app := setupTestApp(t, testBearerToken)
	defer app.teardown()

	body := `{"id":"some-id","timestamp":"t","run_id":"r","event_type":"e","node_id":"","session_id":"","archetype":"","severity":"","payload":{}}`
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

	respBody := make([]byte, 1)
	n, _ := resp.Body.Read(respBody)
	if n != 0 {
		t.Errorf("expected empty response body, got %d bytes", n)
	}

	count := app.eventRowCount(t)
	if count != 0 {
		t.Errorf("expected 0 rows in events table, got %d", count)
	}
}

// TestTS02_E7_ArrayPayloadRejected verifies that an event whose payload
// first non-whitespace byte is '[' (array) is silently rejected.
// Requirement: 02-REQ-2.E3 | Test Spec: TS-02-E7
func TestTS02_E7_ArrayPayloadRejected(t *testing.T) {
	app := setupTestApp(t, testBearerToken)
	defer app.teardown()

	body := `{"id":"arr-id","timestamp":"t","run_id":"r","event_type":"e","node_id":"","session_id":"","archetype":"","severity":"info","payload":["not","an","object"]}`
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

	respBody := make([]byte, 1)
	n, _ := resp.Body.Read(respBody)
	if n != 0 {
		t.Errorf("expected empty response body, got %d bytes", n)
	}

	count := app.eventRowCount(t)
	if count != 0 {
		t.Errorf("expected 0 rows in events table, got %d", count)
	}
}

// TestTS02_E8_NullPayloadRejected verifies that an event whose payload is
// null (first byte 'n') is silently rejected.
// Requirement: 02-REQ-2.E3 | Test Spec: TS-02-E8
func TestTS02_E8_NullPayloadRejected(t *testing.T) {
	app := setupTestApp(t, testBearerToken)
	defer app.teardown()

	body := `{"id":"null-payload-id","timestamp":"t","run_id":"r","event_type":"e","node_id":"","session_id":"","archetype":"","severity":"info","payload":null}`
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

	respBody := make([]byte, 1)
	n, _ := resp.Body.Read(respBody)
	if n != 0 {
		t.Errorf("expected empty response body, got %d bytes", n)
	}

	count := app.eventRowCount(t)
	if count != 0 {
		t.Errorf("expected 0 rows in events table, got %d", count)
	}
}

// TestTS02_E9_EmptyBodyReturns400 verifies that an empty request body
// returns HTTP 400 Bad Request, preserving existing middleware behavior.
// Requirement: 02-REQ-2.E4 | Test Spec: TS-02-E9
func TestTS02_E9_EmptyBodyReturns400(t *testing.T) {
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
}

// TestTS02_8_SilentRejectionIndistinguishable verifies that POST /v1/events
// returns HTTP 201 with a zero-byte body and no Content-Type header for both
// valid and invalid (non-empty) event payloads.
// Requirement: 02-REQ-3.1 | Test Spec: TS-02-8
func TestTS02_8_SilentRejectionIndistinguishable(t *testing.T) {
	app := setupTestApp(t, testBearerToken)
	defer app.teardown()

	bodies := []struct {
		name string
		body string
	}{
		{"valid_event", canonicalAuditEventWithID("ts02-8-valid-id")},
		{"invalid_event", `{"bad":"event"}`},
	}

	for _, tc := range bodies {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, app.Server.URL+"/v1/events", strings.NewReader(tc.body))
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

			// Both must return HTTP 201.
			if resp.StatusCode != http.StatusCreated {
				t.Errorf("expected status 201, got %d", resp.StatusCode)
			}

			// Both must have zero-byte body.
			respBody, _ := io.ReadAll(resp.Body)
			if len(respBody) != 0 {
				t.Errorf("expected zero-byte body, got %d bytes", len(respBody))
			}

			// Both must have no Content-Type header.
			if ct := resp.Header.Get("Content-Type"); ct != "" {
				t.Errorf("expected no Content-Type header, got %q", ct)
			}
		})
	}
}
