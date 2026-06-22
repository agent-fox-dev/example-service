package integration_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/agent-fox/example-service/internal/config"
)

// TestTS01_P1_AuthenticatedRequestsAlwaysPersisted exercises the property that
// for any POST /v1/events request that passes authentication and audit event
// validation, exactly one row is inserted with the payload stored and
// received_at set to UTC time.
// Updated for spec 02: uses valid canonical AuditEvent bodies with distinct
// ids (02-REQ-4.2); uses semantic JSON comparison for payload (02-REQ-4.3).
// Property: 01-PROP-1 | Test Spec: TS-01-P1
// Validates: 01-REQ-1.1, 01-REQ-1.2, 01-REQ-1.3
func TestTS01_P1_AuthenticatedRequestsAlwaysPersisted(t *testing.T) {
	app := setupTestApp(t, testBearerToken)
	defer app.teardown()

	// Generate valid AuditEvent bodies with distinct ids and varied payloads.
	type testCase struct {
		id      string
		payload string
	}
	cases := []testCase{
		{"p1-event-0", `{"key":"simple"}`},
		{"p1-event-1", `{"number":42}`},
		{"p1-event-2", `{"nested":{"deep":{"value":1}}}`},
		{"p1-event-3", `{"unicode":"こんにちは世界"}`},
		{"p1-event-4", `{"empty_object":{}}`},
		{"p1-event-5", `{"bool_val":true}`},
		{"p1-event-6", `{"array_val":[1,2,3]}`},
		{"p1-event-7", `{"mixed":{"a":1,"b":"two"}}`},
		{"p1-event-8", `{"null_in_obj":null}`},
		{"p1-event-9", `{"plan_hash":"xyz789"}`},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("body_%d", i), func(t *testing.T) {
			countBefore := app.eventRowCount(t)
			before := time.Now().UTC().Add(-2 * time.Second)

			body := `{"id":"` + tc.id + `","timestamp":"2026-06-18T11:26:26.527713+00:00","run_id":"20260618_112626_17dd3f","event_type":"run.start","node_id":"","session_id":"","archetype":"","severity":"info","payload":` + tc.payload + `}`
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

			// Assert 201 Created.
			if resp.StatusCode != http.StatusCreated {
				t.Fatalf("expected status 201, got %d", resp.StatusCode)
			}

			// Assert exactly one new row was inserted.
			countAfter := app.eventRowCount(t)
			if countAfter != countBefore+1 {
				t.Errorf("expected row count to increase by 1 (from %d to %d), got %d", countBefore, countBefore+1, countAfter)
			}

			// Query the inserted row by id.
			row := app.queryEventByID(t, tc.id)

			// Assert id matches the submitted event's own id.
			if row.ID != tc.id {
				t.Errorf("expected id %q, got %q", tc.id, row.ID)
			}

			// Assert payload is semantically equivalent JSON.
			var storedPayload, expectedPayload any
			if err := json.Unmarshal([]byte(row.Payload), &storedPayload); err != nil {
				t.Fatalf("stored payload is not valid JSON: %v", err)
			}
			if err := json.Unmarshal([]byte(tc.payload), &expectedPayload); err != nil {
				t.Fatalf("expected payload is not valid JSON: %v", err)
			}
			storedJSON, _ := json.Marshal(storedPayload)
			expectedJSON, _ := json.Marshal(expectedPayload)
			if string(storedJSON) != string(expectedJSON) {
				t.Errorf("payload mismatch:\nstored:   %s\nexpected: %s", storedJSON, expectedJSON)
			}

			// Assert received_at is within the expected UTC time window.
			parsedTime, err := time.Parse(time.RFC3339Nano, row.ReceivedAt)
			if err != nil {
				t.Fatalf("failed to parse received_at %q: %v", row.ReceivedAt, err)
			}
			if parsedTime.Before(before) || parsedTime.After(after) {
				t.Errorf("received_at %v is not within expected range [%v, %v]", parsedTime, before, after)
			}
		})
	}
}

// TestTS01_P2_UnauthenticatedRequestsNeverReachStorage exercises the property
// that for any POST /v1/events request with a missing, malformed, or incorrect
// Authorization header, no row is inserted and the response is always 401.
// Property: 01-PROP-2 | Test Spec: TS-01-P2
// Validates: 01-REQ-2.E1, 01-REQ-2.E2, 01-REQ-2.E3
func TestTS01_P2_UnauthenticatedRequestsNeverReachStorage(t *testing.T) {
	app := setupTestApp(t, testBearerToken)
	defer app.teardown()

	// Generate all variants of invalid Authorization header values.
	invalidAuthHeaders := []struct {
		name  string
		value string // empty string means no Authorization header at all
		set   bool   // whether to set the header
	}{
		{"absent", "", false},
		{"empty_string", "", true},
		{"lowercase_bearer", "bearer " + testBearerToken, true},
		{"uppercase_bearer", "BEARER " + testBearerToken, true},
		{"mixed_case_bearer", "bEaReR " + testBearerToken, true},
		{"double_space", "Bearer  " + testBearerToken, true},
		{"no_space", "Bearer" + testBearerToken, true},
		{"tab_separator", "Bearer\t" + testBearerToken, true},
		{"no_prefix", testBearerToken, true},
		{"wrong_token", "Bearer wrong-token", true},
		{"wrong_scheme_basic", "Basic " + testBearerToken, true},
		{"wrong_scheme_token", "Token " + testBearerToken, true},
		// NOTE: "trailing_space_token" case removed — Go's net/http textproto
		// reader strips trailing whitespace from header lines before the
		// application sees them, making it impossible to detect a trailing
		// space in the Authorization header value. See docs/errata/01_go_http_trailing_whitespace.md.
		{"leading_space_token", "Bearer  " + testBearerToken, true},
	}

	for _, tc := range invalidAuthHeaders {
		t.Run(tc.name, func(t *testing.T) {
			countBefore := app.eventRowCount(t)

			body := `{"event":"test"}`
			req, err := http.NewRequest(http.MethodPost, app.Server.URL+"/v1/events", strings.NewReader(body))
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")
			if tc.set {
				req.Header.Set("Authorization", tc.value)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()

			// Assert 401 Unauthorized.
			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("expected status 401, got %d", resp.StatusCode)
			}

			// Assert no rows were inserted.
			countAfter := app.eventRowCount(t)
			if countAfter != countBefore {
				t.Errorf("row count changed from %d to %d — storage should not be reached", countBefore, countAfter)
			}
		})
	}
}

// TestTS01_P3_InvalidPayloadNeverReachesStorage exercises the property that
// for any POST /v1/events request with a valid bearer token but invalid
// payload (missing Content-Type or empty body), no row is inserted and the
// response is 400 Bad Request.
// Property: 01-PROP-3 | Test Spec: TS-01-P3
// Validates: 01-REQ-1.E1, 01-REQ-1.E2
func TestTS01_P3_InvalidPayloadNeverReachesStorage(t *testing.T) {
	app := setupTestApp(t, testBearerToken)
	defer app.teardown()

	invalidPayloads := []struct {
		name        string
		contentType string
		setCT       bool
		body        string
	}{
		{"absent_content_type", "", false, `{"event":"test"}`},
		{"text_plain_content_type", "text/plain", true, `{"event":"test"}`},
		{"text_html_content_type", "text/html", true, `{"event":"test"}`},
		{"xml_content_type", "application/xml", true, `{"event":"test"}`},
		{"empty_body", "application/json", true, ""},
		{"empty_body_no_ct", "", false, ""},
	}

	for _, tc := range invalidPayloads {
		t.Run(tc.name, func(t *testing.T) {
			countBefore := app.eventRowCount(t)

			req, err := http.NewRequest(http.MethodPost, app.Server.URL+"/v1/events", strings.NewReader(tc.body))
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}
			req.Header.Set("Authorization", "Bearer "+testBearerToken)
			if tc.setCT {
				req.Header.Set("Content-Type", tc.contentType)
			} else {
				req.Header.Del("Content-Type")
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()

			// Assert 400 Bad Request.
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("expected status 400, got %d", resp.StatusCode)
			}

			// Assert no rows were inserted.
			countAfter := app.eventRowCount(t)
			if countAfter != countBefore {
				t.Errorf("row count changed from %d to %d — storage should not be reached", countBefore, countAfter)
			}
		})
	}
}

// TestTS01_P4_ReadinessReflectsDBAvailability exercises the property that for
// any GET /readyz request, the response is 200 OK if and only if the SELECT 1
// query succeeds; otherwise 503 Service Unavailable.
// Property: 01-PROP-4 | Test Spec: TS-01-P4
// Validates: 01-REQ-3.2, 01-REQ-3.E1
func TestTS01_P4_ReadinessReflectsDBAvailability(t *testing.T) {
	// Test with database available.
	t.Run("db_available", func(t *testing.T) {
		app := setupTestApp(t, testBearerToken)
		defer app.teardown()

		resp, err := http.Get(app.Server.URL + "/readyz")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200 when DB is available, got %d", resp.StatusCode)
		}
	})

	// Test with database unavailable.
	t.Run("db_unavailable", func(t *testing.T) {
		app := setupTestAppWithBrokenDB(t, testBearerToken)
		defer app.teardown()

		resp, err := http.Get(app.Server.URL + "/readyz")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("expected status 503 when DB is unavailable, got %d", resp.StatusCode)
		}
	})
}

// TestTS01_P5_StartupRequiresAuthBearerToken exercises the property that for
// any service startup attempt without AUTH_BEARER_TOKEN set, the process exits
// with a non-zero status and emits a descriptive error before binding to any
// port.
// Property: 01-PROP-5 | Test Spec: TS-01-P5
// Validates: 01-REQ-4.2
func TestTS01_P5_StartupRequiresAuthBearerToken(t *testing.T) {
	// Vary other env variables across valid values while keeping
	// AUTH_BEARER_TOKEN absent.
	envCombinations := []struct {
		name     string
		port     string
		dbPath   string
		logLevel string
	}{
		{"defaults_only", "", "", ""},
		{"custom_port", "9090", "", ""},
		{"custom_db_path", "", "/tmp/test.db", ""},
		{"custom_log_level", "", "", "debug"},
		{"all_custom", "3000", "/tmp/test.db", "warn"},
	}

	for _, tc := range envCombinations {
		t.Run(tc.name, func(t *testing.T) {
			// Ensure AUTH_BEARER_TOKEN is unset.
			t.Setenv("AUTH_BEARER_TOKEN", "")

			if tc.port != "" {
				t.Setenv("PORT", tc.port)
			} else {
				t.Setenv("PORT", "")
			}
			if tc.dbPath != "" {
				t.Setenv("DB_PATH", tc.dbPath)
			} else {
				t.Setenv("DB_PATH", "")
			}
			if tc.logLevel != "" {
				t.Setenv("LOG_LEVEL", tc.logLevel)
			} else {
				t.Setenv("LOG_LEVEL", "")
			}

			_, err := config.Load()
			if err == nil {
				t.Error("expected config.Load() to return an error when AUTH_BEARER_TOKEN is absent")
			}

			// Verify the error references AUTH_BEARER_TOKEN.
			if err != nil && !strings.Contains(err.Error(), "AUTH_BEARER_TOKEN") {
				t.Errorf("error message should reference AUTH_BEARER_TOKEN, got: %v", err)
			}
		})
	}
}
