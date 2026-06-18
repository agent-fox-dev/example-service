package db

import (
	"context"
	"database/sql"
)

// Ping executes a SELECT 1 query to verify database connectivity.
func Ping(ctx context.Context, database *sql.DB) error {
	return database.QueryRowContext(ctx, "SELECT 1").Err()
}
