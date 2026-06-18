// Package db provides SQLite database access for the events store.
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	// Import the pure-Go SQLite driver.
	_ "modernc.org/sqlite"
)

// Open initialises the SQLite database at dbPath, creating the schema if needed.
// It ensures the parent directory exists before opening the database file.
func Open(dbPath string) (*sql.DB, error) {
	// Ensure the directory for the database file exists.
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating database directory %q: %w", dir, err)
	}

	database, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database %q: %w", dbPath, err)
	}

	// Create the events table if it does not already exist.
	_, err = database.Exec(`CREATE TABLE IF NOT EXISTS events (
		id TEXT PRIMARY KEY,
		payload TEXT NOT NULL,
		received_at DATETIME NOT NULL
	)`)
	if err != nil {
		database.Close()
		return nil, fmt.Errorf("creating events table: %w", err)
	}

	return database, nil
}
