package db

import (
	"context"
	"database/sql"
)

// InsertEvent stores a raw JSON payload in the events table with a generated UUID and UTC timestamp.
func InsertEvent(ctx context.Context, database *sql.DB, payload string) error {
	// TODO: implement in task group 5
	return nil
}
