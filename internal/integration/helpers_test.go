package integration_test

import (
	"database/sql"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/agent-fox/example-service/internal/handler"
	"github.com/agent-fox/example-service/internal/middleware"

	// Import the pure-Go SQLite driver.
	_ "modernc.org/sqlite"
)

const testBearerToken = "test-secret"

// testApp holds the Echo instance, database, and HTTP test server
// used by integration tests.
type testApp struct {
	Echo   *echo.Echo
	DB     *sql.DB
	Server *httptest.Server
	DBPath string
}

// setupTestApp creates an Echo application wired with all routes, backed by
// a temporary SQLite database. The caller must call teardown() when done.
func setupTestApp(t *testing.T, token string) *testApp {
	t.Helper()

	// Create a temporary directory for the SQLite database.
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_events.db")

	// Open the SQLite database.
	database, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	// Create the events table schema (10-column schema per spec 02).
	_, err = database.Exec(`CREATE TABLE IF NOT EXISTS events (
		id TEXT PRIMARY KEY,
		timestamp TEXT NOT NULL,
		run_id TEXT NOT NULL,
		event_type TEXT NOT NULL,
		node_id TEXT NOT NULL,
		session_id TEXT NOT NULL,
		archetype TEXT NOT NULL,
		severity TEXT NOT NULL,
		payload TEXT NOT NULL,
		received_at DATETIME NOT NULL
	)`)
	if err != nil {
		_ = database.Close()
		t.Fatalf("failed to create events table: %v", err)
	}

	// Build the Echo application with routes and middleware.
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	eventsHandler := &handler.EventsHandler{DB: database}
	livenessHandler := &handler.LivenessHandler{}
	readinessHandler := &handler.ReadinessHandler{DB: database}

	e.POST("/v1/events", eventsHandler.Handle, middleware.BearerAuth(token))
	e.GET("/healthz", livenessHandler.Handle)
	e.GET("/readyz", readinessHandler.Handle)

	server := httptest.NewServer(e)

	return &testApp{
		Echo:   e,
		DB:     database,
		Server: server,
		DBPath: dbPath,
	}
}

// teardown cleans up the test application resources.
func (app *testApp) teardown() {
	app.Server.Close()
	if app.DB != nil {
		_ = app.DB.Close()
	}
}

// eventRowCount returns the number of rows in the events table.
func (app *testApp) eventRowCount(t *testing.T) int {
	t.Helper()
	var count int
	err := app.DB.QueryRow("SELECT COUNT(*) FROM events").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count event rows: %v", err)
	}
	return count
}

// eventRow holds all 10 columns of a row from the events table.
type eventRow struct {
	ID         string
	Timestamp  string
	RunID      string
	EventType  string
	NodeID     string
	SessionID  string
	Archetype  string
	Severity   string
	Payload    string
	ReceivedAt string
}

// queryEventByID returns the full event row for the given id, or fails the test.
func (app *testApp) queryEventByID(t *testing.T, id string) eventRow {
	t.Helper()
	var row eventRow
	err := app.DB.QueryRow(
		"SELECT id, timestamp, run_id, event_type, node_id, session_id, archetype, severity, payload, received_at FROM events WHERE id = ?",
		id,
	).Scan(&row.ID, &row.Timestamp, &row.RunID, &row.EventType, &row.NodeID, &row.SessionID, &row.Archetype, &row.Severity, &row.Payload, &row.ReceivedAt)
	if err != nil {
		t.Fatalf("failed to query event by id %q: %v", id, err)
	}
	return row
}

// setupTestAppWithBrokenDB creates a test app whose database file has been
// removed, simulating a database unavailability scenario.
func setupTestAppWithBrokenDB(t *testing.T, token string) *testApp {
	t.Helper()
	app := setupTestApp(t, token)

	// Close the database and remove the file to simulate unavailability.
	_ = app.DB.Close()
	_ = os.Remove(app.DBPath)

	return app
}
