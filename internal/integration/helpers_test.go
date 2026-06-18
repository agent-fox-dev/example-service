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

	// Create the events table schema.
	_, err = database.Exec(`CREATE TABLE IF NOT EXISTS events (
		id TEXT PRIMARY KEY,
		payload TEXT NOT NULL,
		received_at DATETIME NOT NULL
	)`)
	if err != nil {
		database.Close()
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
		app.DB.Close()
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

// setupTestAppWithBrokenDB creates a test app whose database file has been
// removed, simulating a database unavailability scenario.
func setupTestAppWithBrokenDB(t *testing.T, token string) *testApp {
	t.Helper()
	app := setupTestApp(t, token)

	// Close the database and remove the file to simulate unavailability.
	app.DB.Close()
	os.Remove(app.DBPath)

	return app
}
