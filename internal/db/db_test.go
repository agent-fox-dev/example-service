package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	// Import the pure-Go SQLite driver.
	_ "modernc.org/sqlite"
)

func TestOpen_CreatesDirectoryAndSchema(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "sub", "nested", "test.db")

	database, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = database.Close() }()

	// Verify the directory was created.
	if _, err := os.Stat(filepath.Dir(dbPath)); os.IsNotExist(err) {
		t.Error("expected database directory to be created")
	}

	// Verify the events table exists by querying it.
	var count int
	err = database.QueryRow("SELECT COUNT(*) FROM events").Scan(&count)
	if err != nil {
		t.Fatalf("events table query failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 rows in fresh events table, got %d", count)
	}
}

func TestOpen_IdempotentSchema(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Open twice to verify CREATE TABLE IF NOT EXISTS is idempotent.
	db1, err := Open(dbPath)
	if err != nil {
		t.Fatalf("first Open failed: %v", err)
	}
	_ = db1.Close()

	db2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("second Open failed: %v", err)
	}
	defer func() { _ = db2.Close() }()

	var count int
	err = db2.QueryRow("SELECT COUNT(*) FROM events").Scan(&count)
	if err != nil {
		t.Fatalf("events table query failed after re-open: %v", err)
	}
}

// TestInsertEvent_StoresAuditEventWithReceivedAt verifies that InsertEvent
// stores all 9 event fields and generates a received_at timestamp.
// Updated for spec 02: uses AuditEvent struct, verifies event's own id is
// used as the primary key (no server-side UUID).
// Requirement: 02-REQ-7.2 | Test Spec: TS-02-17
func TestInsertEvent_StoresAuditEventWithReceivedAt(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = database.Close() }()

	event := AuditEvent{
		ID:        "test-id",
		Timestamp: "2026-06-18T11:26:26.527713+00:00",
		RunID:     "20260618_112626_17dd3f",
		EventType: "run.start",
		NodeID:    "",
		SessionID: "",
		Archetype: "",
		Severity:  "info",
		Payload:   json.RawMessage(`{"plan_hash":"abc123"}`),
	}

	before := time.Now().UTC().Add(-2 * time.Second)

	err = InsertEvent(context.Background(), database, event)
	if err != nil {
		t.Fatalf("InsertEvent failed: %v", err)
	}

	after := time.Now().UTC().Add(2 * time.Second)

	var id, ts, runID, eventType, nodeID, sessionID, archetype, severity, payload, receivedAt string
	err = database.QueryRow("SELECT id, timestamp, run_id, event_type, node_id, session_id, archetype, severity, payload, received_at FROM events WHERE id = ?", "test-id").Scan(
		&id, &ts, &runID, &eventType, &nodeID, &sessionID, &archetype, &severity, &payload, &receivedAt,
	)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	// Verify event's own id is used as the primary key.
	if id != "test-id" {
		t.Errorf("expected id %q, got %q", "test-id", id)
	}

	// Verify all event fields are stored correctly.
	if ts != event.Timestamp {
		t.Errorf("expected timestamp %q, got %q", event.Timestamp, ts)
	}
	if runID != event.RunID {
		t.Errorf("expected run_id %q, got %q", event.RunID, runID)
	}
	if eventType != event.EventType {
		t.Errorf("expected event_type %q, got %q", event.EventType, eventType)
	}
	if nodeID != event.NodeID {
		t.Errorf("expected node_id %q, got %q", event.NodeID, nodeID)
	}
	if sessionID != event.SessionID {
		t.Errorf("expected session_id %q, got %q", event.SessionID, sessionID)
	}
	if archetype != event.Archetype {
		t.Errorf("expected archetype %q, got %q", event.Archetype, archetype)
	}
	if severity != event.Severity {
		t.Errorf("expected severity %q, got %q", event.Severity, severity)
	}

	// Verify payload is parseable as JSON with correct content.
	var parsedPayload map[string]any
	if err := json.Unmarshal([]byte(payload), &parsedPayload); err != nil {
		t.Fatalf("stored payload is not valid JSON: %v", err)
	}
	if parsedPayload["plan_hash"] != "abc123" {
		t.Errorf("expected payload plan_hash %q, got %v", "abc123", parsedPayload["plan_hash"])
	}

	// Verify received_at is a recent UTC timestamp parseable as RFC3339Nano.
	parsedTime, err := time.Parse(time.RFC3339Nano, receivedAt)
	if err != nil {
		t.Fatalf("failed to parse received_at %q: %v", receivedAt, err)
	}
	if parsedTime.Before(before) || parsedTime.After(after) {
		t.Errorf("received_at %v not within [%v, %v]", parsedTime, before, after)
	}
}

// TestInsertEvent_ReturnsErrorOnMissingTable verifies that InsertEvent returns
// a non-nil error when the events table does not exist.
func TestInsertEvent_ReturnsErrorOnMissingTable(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = database.Close() }()

	// Drop the events table.
	_, err = database.Exec("DROP TABLE events")
	if err != nil {
		t.Fatalf("failed to drop events table: %v", err)
	}

	event := AuditEvent{
		ID:        "test-missing-table",
		Timestamp: "t",
		RunID:     "r",
		EventType: "e",
		NodeID:    "",
		SessionID: "",
		Archetype: "",
		Severity:  "info",
		Payload:   json.RawMessage(`{}`),
	}

	err = InsertEvent(context.Background(), database, event)
	if err == nil {
		t.Error("expected InsertEvent to return error when table is missing")
	}
}

func TestPing_SucceedsWithOpenDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = database.Close() }()

	if err := Ping(context.Background(), database); err != nil {
		t.Errorf("Ping failed with open database: %v", err)
	}
}

func TestPing_FailsWithClosedDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	_ = database.Close()

	err = Ping(context.Background(), database)
	if err == nil {
		t.Error("expected Ping to fail with closed database")
	}
}

// --- Spec 02 Unit Tests ---

// TestTS02_1_AuditEventStructFields verifies that the AuditEvent struct
// defines exactly 9 exported fields with correct Go types and JSON struct tags.
// Requirement: 02-REQ-1.1 | Test Spec: TS-02-1
func TestTS02_1_AuditEventStructFields(t *testing.T) {
	typ := reflect.TypeOf(AuditEvent{})

	// Assert exactly 9 fields.
	if typ.NumField() != 9 {
		t.Fatalf("expected AuditEvent to have 9 fields, got %d", typ.NumField())
	}

	// Expected fields: name, Go type, JSON tag.
	expected := []struct {
		name    string
		goType  string
		jsonTag string
	}{
		{"ID", "string", "id"},
		{"Timestamp", "string", "timestamp"},
		{"RunID", "string", "run_id"},
		{"EventType", "string", "event_type"},
		{"NodeID", "string", "node_id"},
		{"SessionID", "string", "session_id"},
		{"Archetype", "string", "archetype"},
		{"Severity", "string", "severity"},
		{"Payload", "json.RawMessage", "payload"},
	}

	for _, exp := range expected {
		field, ok := typ.FieldByName(exp.name)
		if !ok {
			t.Errorf("AuditEvent missing field %q", exp.name)
			continue
		}

		// Check Go type.
		gotType := field.Type.String()
		if gotType != exp.goType {
			t.Errorf("field %q: expected type %q, got %q", exp.name, exp.goType, gotType)
		}

		// Check JSON struct tag.
		tag := field.Tag.Get("json")
		// JSON tag may have options (e.g. "id,omitempty"), so compare just the name part.
		tagName := strings.Split(tag, ",")[0]
		if tagName != exp.jsonTag {
			t.Errorf("field %q: expected json tag %q, got %q", exp.name, exp.jsonTag, tagName)
		}
	}
}

// TestTS02_2_PayloadIsJSONRawMessage verifies that the Payload field of
// AuditEvent is declared as json.RawMessage and preserves raw JSON bytes
// during unmarshalling.
// Requirement: 02-REQ-1.2 | Test Spec: TS-02-2
func TestTS02_2_PayloadIsJSONRawMessage(t *testing.T) {
	body := `{"id":"abc","timestamp":"t","run_id":"r","event_type":"e","node_id":"","session_id":"","archetype":"","severity":"info","payload":{"k":"v"}}`
	var event AuditEvent
	if err := json.Unmarshal([]byte(body), &event); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	// Verify the Go type of Payload is json.RawMessage.
	payloadType := reflect.TypeOf(event.Payload)
	if payloadType.String() != "json.RawMessage" {
		t.Errorf("expected Payload type json.RawMessage, got %q", payloadType.String())
	}

	// Verify raw bytes are preserved.
	payloadStr := string(event.Payload)
	if payloadStr != `{"k":"v"}` {
		t.Errorf("expected Payload bytes %q, got %q", `{"k":"v"}`, payloadStr)
	}
}

// TestTS02_9_EventsTableSchemaHas10Columns verifies that PRAGMA table_info(events)
// returns exactly 10 columns with the correct names, types, and constraints.
// Requirement: 02-REQ-4.1 | Test Spec: TS-02-9
func TestTS02_9_EventsTableSchemaHas10Columns(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = database.Close() }()

	rows, err := database.Query("PRAGMA table_info(events)")
	if err != nil {
		t.Fatalf("PRAGMA table_info failed: %v", err)
	}
	defer func() { _ = rows.Close() }()

	type columnInfo struct {
		CID       int
		Name      string
		Type      string
		NotNull   int
		DfltValue *string
		PK        int
	}

	var columns []columnInfo
	for rows.Next() {
		var col columnInfo
		if err := rows.Scan(&col.CID, &col.Name, &col.Type, &col.NotNull, &col.DfltValue, &col.PK); err != nil {
			t.Fatalf("failed to scan column info: %v", err)
		}
		columns = append(columns, col)
	}

	// Assert exactly 10 columns.
	if len(columns) != 10 {
		t.Fatalf("expected 10 columns, got %d", len(columns))
	}

	// Build a map for easy lookup.
	colMap := make(map[string]columnInfo)
	for _, col := range columns {
		colMap[col.Name] = col
	}

	// Verify id is PRIMARY KEY.
	if colMap["id"].PK != 1 {
		t.Errorf("expected id to be PRIMARY KEY (pk=1), got pk=%d", colMap["id"].PK)
	}

	// Verify NOT NULL constraints on all non-PK columns.
	notNullCols := []string{"timestamp", "run_id", "event_type", "node_id", "session_id", "archetype", "severity", "payload", "received_at"}
	for _, name := range notNullCols {
		col, ok := colMap[name]
		if !ok {
			t.Errorf("missing column %q", name)
			continue
		}
		if col.NotNull != 1 {
			t.Errorf("column %q: expected NOT NULL (notnull=1), got notnull=%d", name, col.NotNull)
		}
	}

	// Verify received_at type is DATETIME.
	if colMap["received_at"].Type != "DATETIME" {
		t.Errorf("expected received_at type DATETIME, got %q", colMap["received_at"].Type)
	}

	// Verify other TEXT columns.
	textCols := []string{"id", "timestamp", "run_id", "event_type", "node_id", "session_id", "archetype", "severity", "payload"}
	for _, name := range textCols {
		if colMap[name].Type != "TEXT" {
			t.Errorf("column %q: expected type TEXT, got %q", name, colMap[name].Type)
		}
	}
}

// TestTS02_13_DDLCreatesTableOnFreshDB verifies that on startup the DB
// initialization function executes CREATE TABLE IF NOT EXISTS events
// and creates the 10-column schema on a fresh database.
// Requirement: 02-REQ-5.1 | Test Spec: TS-02-13
func TestTS02_13_DDLCreatesTableOnFreshDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "fresh.db")

	database, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = database.Close() }()

	// Verify events table exists with 10 columns.
	rows, err := database.Query("PRAGMA table_info(events)")
	if err != nil {
		t.Fatalf("PRAGMA table_info failed: %v", err)
	}
	defer func() { _ = rows.Close() }()

	count := 0
	for rows.Next() {
		count++
		var cid, notnull, pk int
		var name, colType string
		var dflt *string
		if err := rows.Scan(&cid, &name, &colType, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan failed: %v", err)
		}
	}
	if count != 10 {
		t.Errorf("expected 10 columns after fresh Open, got %d", count)
	}
}

// TestTS02_14_NoDROPTableInSource verifies that the source of internal/db/db.go
// does not contain any DROP TABLE statement.
// Requirement: 02-REQ-5.2 | Test Spec: TS-02-14
func TestTS02_14_NoDROPTableInSource(t *testing.T) {
	source, err := os.ReadFile("db.go")
	if err != nil {
		t.Fatalf("failed to read db.go: %v", err)
	}

	upper := strings.ToUpper(string(source))
	if strings.Contains(upper, "DROP TABLE") {
		t.Error("db.go contains DROP TABLE statement, which is prohibited by 02-REQ-5.2")
	}
}

// TestTS02_16_InsertEventSignature verifies that InsertEvent has the correct
// function signature: func InsertEvent(ctx context.Context, db *sql.DB, event AuditEvent) error.
// Requirement: 02-REQ-7.1 | Test Spec: TS-02-16
func TestTS02_16_InsertEventSignature(t *testing.T) {
	// Compile-time check: this assignment must type-check with the expected
	// signature. If InsertEvent's signature changes, this test won't compile.
	var fn func(context.Context, *sql.DB, AuditEvent) error
	fn = InsertEvent
	// Use fn to prevent "declared and not used" error.
	_ = fn
}

// TestTS02_E11_OldSchemaNoConflict verifies that if the events table already
// exists (even with the old 3-column schema), CREATE TABLE IF NOT EXISTS is a
// no-op and the startup proceeds without error.
// Requirement: 02-REQ-5.E2 | Test Spec: TS-02-E11
func TestTS02_E11_OldSchemaNoConflict(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "old_schema.db")

	// Create a database with the old 3-column schema.
	database, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	_, err = database.Exec(`CREATE TABLE events (
		id TEXT PRIMARY KEY,
		payload TEXT NOT NULL,
		received_at DATETIME NOT NULL
	)`)
	if err != nil {
		_ = database.Close()
		t.Fatalf("failed to create old schema: %v", err)
	}
	_ = database.Close()

	// Now call Open (which runs CREATE TABLE IF NOT EXISTS with the new schema).
	// This should succeed as a no-op because the table already exists.
	database2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open should succeed with pre-existing table (IF NOT EXISTS is a no-op), got: %v", err)
	}
	defer func() { _ = database2.Close() }()
}

// TestTS02_E10_DDLFailureCausesFatalExit verifies that if the CREATE TABLE
// IF NOT EXISTS DDL fails, the service returns an error (which triggers
// log.Fatal / non-zero exit in the caller).
// Requirement: 02-REQ-5.E1 | Test Spec: TS-02-E10
//
// Note: Testing log.Fatal/os.Exit requires subprocess invocation since
// os.Exit terminates the test binary. This test verifies that Open returns
// an error when DDL execution fails (the wrapper logs fatal and exits).
func TestTS02_E10_DDLFailureCausesFatalExit(t *testing.T) {
	// Create a read-only directory to prevent the database file from being
	// written, which will cause the DDL to fail.
	tmpDir := t.TempDir()
	roDir := filepath.Join(tmpDir, "readonly")
	if err := os.MkdirAll(roDir, 0o755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	// Create a database file and perform a write to materialise it on disk.
	dbPath := filepath.Join(roDir, "test.db")
	database, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	// Ping to force the driver to actually create the file.
	if err := database.Ping(); err != nil {
		_ = database.Close()
		t.Fatalf("failed to ping database: %v", err)
	}
	_ = database.Close()

	// Make the database file read-only to force DDL failure.
	if err := os.Chmod(dbPath, 0o444); err != nil {
		t.Fatalf("failed to chmod database file: %v", err)
	}
	defer func() { _ = os.Chmod(dbPath, 0o644) }()

	// Open should fail because DDL cannot execute on a read-only database.
	db2, err := Open(dbPath)
	if err == nil {
		_ = db2.Close()
		t.Error("expected Open to return error when DDL fails on read-only database")
	}
}
