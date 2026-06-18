package integration_test

import (
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/agent-fox/example-service/internal/config"
)

// TestTS01_P1_AuthenticatedRequestsAlwaysPersisted exercises the property that
// for any POST /v1/events request that passes authentication and payload
// validation, exactly one row is inserted with the raw payload stored and
// received_at set to UTC time.
// Property: 01-PROP-1 | Test Spec: TS-01-P1
// Validates: 01-REQ-1.1, 01-REQ-1.2, 01-REQ-1.3
func TestTS01_P1_AuthenticatedRequestsAlwaysPersisted(t *testing.T) {
	app := setupTestApp(t, testBearerToken)
	defer app.teardown()

	// Generate a variety of arbitrary non-empty JSON bodies.
	testBodies := []string{
		`{"simple":"value"}`,
		`{"number":42}`,
		`{"bool":true}`,
		`{"null_field":null}`,
		`{"nested":{"deep":{"value":1}}}`,
		`{"array":[1,2,3,4,5]}`,
		`{"mixed":{"a":1,"b":"two","c":[true,false],"d":null}}`,
		`{"unicode":"こんにちは世界"}`,
		`{"empty_object":{}}`,
		`{"empty_array":[]}`,
	}

	// Add some randomly generated bodies.
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("prop_key_%d", i)
		val := rng.Intn(10000)
		testBodies = append(testBodies, fmt.Sprintf(`{"%s":%d}`, key, val))
	}

	for i, body := range testBodies {
		t.Run(fmt.Sprintf("body_%d", i), func(t *testing.T) {
			countBefore := app.eventRowCount(t)
			before := time.Now().UTC().Add(-2 * time.Second)

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
			defer resp.Body.Close()

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

			// Query the most recently inserted row.
			var id, payload, receivedAt string
			err = app.DB.QueryRow("SELECT id, payload, received_at FROM events ORDER BY rowid DESC LIMIT 1").Scan(&id, &payload, &receivedAt)
			if err != nil {
				t.Fatalf("failed to query event row: %v", err)
			}

			// Assert payload matches the exact body sent.
			if payload != body {
				t.Errorf("payload mismatch:\nexpected: %s\ngot:      %s", body, payload)
			}

			// Assert id is a valid UUID.
			if _, err := uuid.Parse(id); err != nil {
				t.Errorf("id %q is not a valid UUID: %v", id, err)
			}

			// Assert received_at is within the expected UTC time window.
			parsedTime, err := time.Parse(time.RFC3339Nano, receivedAt)
			if err != nil {
				parsedTime, err = time.Parse("2006-01-02T15:04:05Z", receivedAt)
				if err != nil {
					parsedTime, err = time.Parse("2006-01-02 15:04:05", receivedAt)
					if err != nil {
						t.Fatalf("failed to parse received_at %q: %v", receivedAt, err)
					}
				}
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
		{"trailing_space_token", "Bearer " + testBearerToken + " ", true},
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
			defer resp.Body.Close()

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
			defer resp.Body.Close()

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
		defer resp.Body.Close()

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
		defer resp.Body.Close()

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
