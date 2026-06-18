package db

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// InsertEvent stores a raw JSON payload in the events table with a generated UUID and UTC timestamp.
func InsertEvent(ctx context.Context, database *sql.DB, payload string) error {
	id := uuid.New().String()
	receivedAt := time.Now().UTC().Format(time.RFC3339Nano)

	_, err := database.ExecContext(ctx,
		"INSERT INTO events (id, payload, received_at) VALUES (?, ?, ?)",
		id, payload, receivedAt,
	)
	return err
}
