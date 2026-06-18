package integration_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestTS01_E5_MissingAuthorizationHeader verifies that POST /v1/events with a
// missing Authorization header returns 401 Unauthorized and does not write to
// the events table.
// Requirement: 01-REQ-2.E1 | Test Spec: TS-01-E5
func TestTS01_E5_MissingAuthorizationHeader(t *testing.T) {
	app := setupTestApp(t, testBearerToken)
	defer app.teardown()

	body := `{"event":"test"}`
	req, err := http.NewRequest(http.MethodPost, app.Server.URL+"/v1/events", strings.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// Deliberately not setting Authorization header.

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", resp.StatusCode)
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

// TestTS01_E6_MalformedAuthorizationHeader verifies that a malformed
// Authorization header (wrong casing 'bearer', extra whitespace, missing
// 'Bearer ' prefix, all-caps) returns 401 Unauthorized and does not write
// to the events table.
// Requirement: 01-REQ-2.E2 | Test Spec: TS-01-E6
func TestTS01_E6_MalformedAuthorizationHeader(t *testing.T) {
	app := setupTestApp(t, testBearerToken)
	defer app.teardown()

	malformedHeaders := []struct {
		name  string
		value string
	}{
		{"lowercase_bearer", "bearer " + testBearerToken},
		{"double_space", "Bearer  " + testBearerToken},
		{"no_prefix", testBearerToken},
		{"uppercase_bearer", "BEARER " + testBearerToken},
	}

	for _, tc := range malformedHeaders {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"e":1}`
			req, err := http.NewRequest(http.MethodPost, app.Server.URL+"/v1/events", strings.NewReader(body))
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}
			req.Header.Set("Authorization", tc.value)
			req.Header.Set("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("expected status 401 for Authorization %q, got %d", tc.value, resp.StatusCode)
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
				t.Errorf("response body does not contain 'message' field for Authorization %q", tc.value)
			}
		})
	}

	// Verify no rows were inserted across all malformed header attempts.
	count := app.eventRowCount(t)
	if count != 0 {
		t.Errorf("expected 0 rows in events table after all malformed auth attempts, got %d", count)
	}
}

// TestTS01_E7_WrongTokenValue verifies that a well-formed Authorization header
// with an incorrect token value returns 401 Unauthorized and does not write to
// the events table.
// Requirement: 01-REQ-2.E3 | Test Spec: TS-01-E7
func TestTS01_E7_WrongTokenValue(t *testing.T) {
	app := setupTestApp(t, "correct-secret")
	defer app.teardown()

	body := `{"event":"test"}`
	req, err := http.NewRequest(http.MethodPost, app.Server.URL+"/v1/events", strings.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer wrong-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", resp.StatusCode)
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
