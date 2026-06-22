package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// AuditEvent represents a validated audit event with JSON struct tags for
// unmarshalling. It defines exactly 9 top-level fields per spec 02-REQ-1.1.
type AuditEvent struct {
	ID        string          `json:"id"`
	Timestamp string          `json:"timestamp"`
	RunID     string          `json:"run_id"`
	EventType string          `json:"event_type"`
	NodeID    string          `json:"node_id"`
	SessionID string          `json:"session_id"`
	Archetype string          `json:"archetype"`
	Severity  string          `json:"severity"`
	Payload   json.RawMessage `json:"payload"`
}

// InsertEvent stores a validated AuditEvent in the events table using
// INSERT OR IGNORE to silently handle duplicate id conflicts.
func InsertEvent(ctx context.Context, database *sql.DB, event AuditEvent) error {
	receivedAt := time.Now().UTC().Format(time.RFC3339Nano)

	payloadBytes, err := json.Marshal(event.Payload)
	if err != nil {
		return err
	}

	_, err = database.ExecContext(ctx,
		"INSERT OR IGNORE INTO events (id, timestamp, run_id, event_type, node_id, session_id, archetype, severity, payload, received_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		event.ID, event.Timestamp, event.RunID, event.EventType, event.NodeID, event.SessionID, event.Archetype, event.Severity, string(payloadBytes), receivedAt,
	)
	return err
}
