package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

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
	defer database.Close()

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
	db1.Close()

	db2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("second Open failed: %v", err)
	}
	defer db2.Close()

	var count int
	err = db2.QueryRow("SELECT COUNT(*) FROM events").Scan(&count)
	if err != nil {
		t.Fatalf("events table query failed after re-open: %v", err)
	}
}

func TestInsertEvent_StoresPayloadWithUUIDAndTimestamp(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer database.Close()

	payload := `{"key":"value","nested":{"a":1}}`
	before := time.Now().UTC().Add(-2 * time.Second)

	err = InsertEvent(context.Background(), database, payload)
	if err != nil {
		t.Fatalf("InsertEvent failed: %v", err)
	}

	after := time.Now().UTC().Add(2 * time.Second)

	var id, storedPayload, receivedAt string
	err = database.QueryRow("SELECT id, payload, received_at FROM events").Scan(&id, &storedPayload, &receivedAt)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	// Verify payload stored verbatim.
	if storedPayload != payload {
		t.Errorf("expected payload %q, got %q", payload, storedPayload)
	}

	// Verify id is a valid UUID.
	if _, err := uuid.Parse(id); err != nil {
		t.Errorf("id %q is not a valid UUID: %v", id, err)
	}

	// Verify received_at is within the expected time window.
	parsedTime, err := time.Parse(time.RFC3339Nano, receivedAt)
	if err != nil {
		t.Fatalf("failed to parse received_at %q: %v", receivedAt, err)
	}
	if parsedTime.Before(before) || parsedTime.After(after) {
		t.Errorf("received_at %v not within [%v, %v]", parsedTime, before, after)
	}
}

func TestInsertEvent_ReturnsErrorOnMissingTable(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer database.Close()

	// Drop the events table.
	_, err = database.Exec("DROP TABLE events")
	if err != nil {
		t.Fatalf("failed to drop events table: %v", err)
	}

	err = InsertEvent(context.Background(), database, `{"test":1}`)
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
	defer database.Close()

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
	database.Close()

	err = Ping(context.Background(), database)
	if err == nil {
		t.Error("expected Ping to fail with closed database")
	}
}
