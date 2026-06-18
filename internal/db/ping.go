package db

import (
	"context"
	"database/sql"
)

// Ping executes a SELECT 1 query to verify database connectivity.
func Ping(ctx context.Context, database *sql.DB) error {
	// TODO: implement in task group 5
	return nil
}
