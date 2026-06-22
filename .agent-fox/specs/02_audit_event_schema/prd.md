---
spec_id: '02'
spec_name: audit_event_schema
title: Audit Event Schema
status: draft
created_at: '2026-06-22T12:00:26.030205+00:00'
updated_at: '2026-06-22T12:07:13.079285+00:00'
owner: agent-fox team
source: .agent-fox/audit/audit_20260618_112626_17dd3f.jsonl
schema_version: 1
---
# Audit Event Schema Enforcement

## Intent

Update the existing event ingestion endpoint and database schema to accept only
properly structured audit events from agent-fox, replacing the current raw JSON
passthrough with structured validation and typed storage.

## Background

The service currently accepts any arbitrary JSON payload on `POST /v1/events`
and stores it as raw text in a simple three-column table (`id`, `payload`,
`received_at`). This must be changed to validate incoming events against the
agent-fox audit event structure and store each field in a dedicated column.
All other aspects of the service (health checks, authentication, configuration,
logging, graceful shutdown) remain unchanged.

## Goals

- The `POST /v1/events` endpoint validates that incoming JSON matches the
  agent-fox audit event structure before storing.
- The `events` database table schema has dedicated columns for every top-level
  field in an audit event.
- Events that do not conform to the expected structure are silently rejected
  (return 201 Created, do not store) to prevent denial-of-service via error
  flooding or schema probing.

## Non-Goals

- No changes to authentication, health checks, configuration, logging, or
  graceful shutdown behaviour.
- No query or retrieval API for stored events.
- No payload sub-schema validation (the `payload` field is stored as raw JSON
  text regardless of event type).
- No event type enumeration enforcement — `event_type` accepts any non-empty
  string to remain forward-compatible with future event types.
- No metrics or observability signal for rejected events in this spec —
  observability for rejected events may be addressed in a follow-up spec if
  operational need arises.
- No request body size limit in this spec — rely on Echo framework defaults;
  a body size limit may be addressed in a follow-up spec if needed.

## Functional Requirements

### Audit Event Structure

Every audit event is a JSON object with exactly the following 9 top-level
fields:

| Field        | JSON Type | Constraints                                              |
|--------------|-----------|----------------------------------------------------------|
| `id`         | string    | Required, non-empty string (no UUID format enforcement). |
| `timestamp`  | string    | Required, non-empty.                                     |
| `run_id`     | string    | Required, non-empty.                                     |
| `event_type` | string    | Required, non-empty.                                     |
| `node_id`    | string    | Required (may be empty string `""`).                     |
| `session_id` | string    | Required (may be empty string `""`).                     |
| `archetype`  | string    | Required (may be empty string `""`).                     |
| `severity`   | string    | Required, non-empty.                                     |
| `payload`    | object    | Required, must be a JSON object (may be empty `{}`).     |

> **Note on `id` format:** The `id` field is validated as a non-empty string
> only. No UUID format check is performed. This ensures forward compatibility
> if the identifier format changes in future agent-fox versions.

### Validation Rules

The handler must validate each incoming JSON body against the audit event
structure. Validation checks:

1. The body is valid JSON and parses into an object (not an array or scalar).
2. All 9 top-level fields listed above are present.
3. String fields (`id`, `timestamp`, `run_id`, `event_type`, `node_id`,
   `session_id`, `archetype`, `severity`) have JSON string type.
4. Fields marked "non-empty" in the table above must not be empty strings.
5. The `payload` field has JSON object type.
6. Extra top-level fields beyond the 9 listed are **silently ignored** and are
   not cause for rejection. Do **not** use `json.Decoder` with
   `DisallowUnknownFields`. Go's default `json.Unmarshal` behaviour (ignoring
   unknown fields) is the correct and intended approach. The agent-fox event
   format may add new fields in future versions, and the service must tolerate
   them.

Events failing any validation check are **silently rejected**: the handler
returns `201 Created` with a zero-byte body and no `Content-Type` header
(identical to the success response, implemented via `c.NoContent(http.StatusCreated)`)
but does not store the event. No error is logged for rejected events to avoid
log flooding in a DoS scenario. No metrics or other observability signals are
emitted for rejected events within this spec.

#### Validation Implementation

**Step 1 — Single-pass struct unmarshal (covers checks 1–3):**
Unmarshal the request body directly into an `AuditEvent` struct using
`json.Unmarshal`. Do **not** use a two-pass approach. If `json.Unmarshal`
returns an error for any reason — including the body being an array, a scalar,
`null`, or malformed JSON — treat it as a silent rejection and return
`201 Created` without storing. Go's `json.Unmarshal` into a struct already
fails when the top-level JSON value is not an object, which is exactly the
desired behaviour for check 1.

**Step 2 — Non-empty string checks (check 4):**
After successful unmarshal, verify that the following fields are non-empty
strings: `id`, `timestamp`, `run_id`, `event_type`, `severity`. Fields
`node_id`, `session_id`, and `archetype` may be empty strings and must not
be rejected on that basis.

**Step 3 — Payload object type check (check 5):**
After unmarshal, inspect the `Payload` field (a `json.RawMessage`). Trim
leading whitespace from the raw bytes and check that the first byte is `'{'`.
If it is not (e.g., `[`, `n` for null, `"`, or a digit), treat it as a silent
rejection. This is the simplest and fastest correct approach; no secondary
unmarshal is needed.

#### Body Parsing and Silent Rejection Boundary

| Incoming body          | Response            | Notes                                      |
|------------------------|---------------------|--------------------------------------------|
| Empty body             | `400 Bad Request`   | Preserved existing behaviour (unchanged).  |
| Non-empty, invalid JSON (e.g. plain text, truncated JSON) | `201 Created` (silent rejection) | Treated as an invalid event; not stored.   |
| Non-empty, valid JSON but fails audit event validation    | `201 Created` (silent rejection) | Treated as an invalid event; not stored.   |
| Non-empty, valid JSON, passes all validation              | `201 Created`       | Event stored in `events` table.            |

The **only** case that returns `400` is an empty body, which is already handled
by the existing middleware and is unchanged by this spec.

### HTTP Response Contract

`POST /v1/events` returns the following response for **all** outcomes (both
accepted and silently rejected events):

| Property          | Value                              |
|-------------------|------------------------------------|
| HTTP status code  | `201 Created`                      |
| Response body     | Zero bytes (empty body)            |
| `Content-Type`    | Not set (no `Content-Type` header) |

This is implemented via `c.NoContent(http.StatusCreated)` and matches the
existing handler behaviour. Returning an identical response for valid and
invalid events prevents attackers from distinguishing accepted from rejected
events.

### Updated Database Schema

The `events` table must be updated to a new schema that has a dedicated
column for each top-level audit event field, plus a server-side timestamp.

#### Migration Strategy

Use `CREATE TABLE IF NOT EXISTS` with the new schema DDL on service startup.
**Do not use `DROP TABLE`.**

If the table already exists (e.g., from a previous deployment with the old
three-column schema), the service continues to start normally — SQLite does not
enforce strict column type validation and the old schema will not conflict with
the startup DDL due to `IF NOT EXISTS`. For a clean migration from the old
schema, the database file must be deleted manually before restarting the
service. This is acceptable given the non-production nature of the service and
avoids any risk of destructive data loss from an automated drop on startup.

> **Rationale for not using DROP TABLE:** Automatically dropping the table on
> every startup would be destructive in any scenario where two instances start
> simultaneously (e.g., a rolling restart or blue-green deployment), and would
> make accidental data loss trivially easy. The test/integration context does
> not justify this risk when `CREATE TABLE IF NOT EXISTS` achieves a safe and
> sufficient result.

**Startup failure handling:** If the `CREATE TABLE IF NOT EXISTS` DDL
statement fails (e.g., due to a syntax error or disk issue), the service must
log a fatal error and exit (refuse to start). This matches the existing
behaviour where schema creation failure causes the service to exit. The service
must never continue running without a valid `events` table, as every subsequent
insert would fail.

#### Schema Definition

| Column        | Type     | Constraints                                        |
|---------------|----------|----------------------------------------------------|
| `id`          | TEXT     | Primary key, from the event's `id` field           |
| `timestamp`   | TEXT     | NOT NULL, from the event's `timestamp` field       |
| `run_id`      | TEXT     | NOT NULL                                           |
| `event_type`  | TEXT     | NOT NULL                                           |
| `node_id`     | TEXT     | NOT NULL (may be empty string)                     |
| `session_id`  | TEXT     | NOT NULL (may be empty string)                     |
| `archetype`   | TEXT     | NOT NULL (may be empty string)                     |
| `severity`    | TEXT     | NOT NULL                                           |
| `payload`     | TEXT     | NOT NULL, re-serialized JSON text of the payload object (see note below) |
| `received_at` | DATETIME | NOT NULL, server-side UTC timestamp set on insert, RFC3339 with nanosecond precision (e.g. `2026-06-18T11:26:26.123456789Z`), stored using `time.RFC3339Nano` |

> **Payload serialization note:** The `payload` field is stored as the
> re-serialized JSON output from the parsed Go value (i.e., via
> `json.Marshal` on the parsed `json.RawMessage` or equivalent). Exact
> byte-for-byte preservation of the original request representation is not
> guaranteed (key order or whitespace may differ); downstream consumers are
> expected to parse the stored TEXT as JSON and must not rely on a specific
> byte representation.

> **`received_at` format note:** Values are formatted with `time.RFC3339Nano`
> (e.g., `2026-06-18T11:26:26.123456789Z`), matching the existing behaviour
> of the current `InsertEvent` function. No uniqueness or ordering constraint
> is placed on `received_at`. The `DATETIME` column type is used consistently
> with the existing codebase; SQLite stores it as TEXT under its type affinity
> rules, which is intentional and expected.

The `id` from the event itself becomes the primary key. The server no longer
generates UUIDs for events.

#### Explicit DDL

The following DDL is the authoritative definition of the new `events` table.
Implementers must use this statement exactly:

```sql
CREATE TABLE IF NOT EXISTS events (
    id          TEXT     PRIMARY KEY,
    timestamp   TEXT     NOT NULL,
    run_id      TEXT     NOT NULL,
    event_type  TEXT     NOT NULL,
    node_id     TEXT     NOT NULL,
    session_id  TEXT     NOT NULL,
    archetype   TEXT     NOT NULL,
    severity    TEXT     NOT NULL,
    payload     TEXT     NOT NULL,
    received_at DATETIME NOT NULL
);
```

No `WITHOUT ROWID`, `STRICT`, collation, or index options are applied. Standard
SQLite defaults apply.

### Duplicate Event Handling

If an event with the same `id` is submitted more than once, the duplicate
is silently ignored (`INSERT OR IGNORE`). The handler still returns `201 Created`
with a zero-byte body and no `Content-Type` header.

### Database Insert — Transaction and Handle Injection

**Transaction:** The insert for a single audit event is executed as a bare
`db.ExecContext` call (no explicit transaction wrapper). A single-row insert is
atomic by definition in SQLite and does not require a transaction.

**Handler access to `*sql.DB`:** The existing codebase uses dependency injection
via a handler struct. The `EventsHandler` struct already has a `DB *sql.DB`
field. The updated handler must use this existing pattern — the `*sql.DB` handle
is accessed via `h.DB` (where `h` is the `EventsHandler` receiver). No new
injection mechanism is introduced.

### Unchanged Behaviour

All existing behaviour not related to event structure validation and storage
is preserved:

- Bearer token authentication on `POST /v1/events`
- Content-Type validation (must be `application/json`)
- Empty body rejection (400 Bad Request)
- Health check endpoints (`GET /healthz`, `GET /readyz`)
- Configuration via environment variables
- Structured JSON logging
- Graceful shutdown on SIGTERM/SIGINT
- Error responses use Echo Framework default format

### `AuditEvent` Struct and `InsertEvent` Function

Both the `AuditEvent` struct and the updated `InsertEvent` function live in the
existing **`db` package** (`internal/db`). This package already owns the events
schema and insert logic; no new package is required.

#### `AuditEvent` Struct

The `AuditEvent` struct includes JSON struct tags so that it can be used
directly for `json.Unmarshal` in both the handler and the `db` package without
a separate DTO:

```go
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
```

The `Payload` field is `json.RawMessage` to preserve the raw JSON bytes during
unmarshalling. It is re-serialized via `json.Marshal` when stored to the
database.

#### Updated Function Signature

```go
func InsertEvent(ctx context.Context, db *sql.DB, event AuditEvent) error
```

- **Parameters:**
  - `ctx context.Context` — request context for query cancellation/timeout.
  - `db *sql.DB` — database connection handle.
  - `event AuditEvent` — the fully validated audit event to insert.
- **Return value:** `error` — non-nil if the INSERT fails for a reason other
  than a duplicate primary key. A duplicate `id` (caught by `INSERT OR IGNORE`)
  is not an error; the function returns `nil` in that case.
- **Behaviour:** Executes `INSERT OR IGNORE INTO events (...)` with all 9 event
  fields plus a server-generated `received_at` timestamp
  (`time.Now().UTC().Format(time.RFC3339Nano)`). No explicit transaction wrapper
  is used; the single-row insert is atomic by definition.

All existing call sites of `db.InsertEvent` must be updated to pass an
`AuditEvent` struct.

## Integration Test Fixtures

The following canonical payloads must be used in the new integration tests to
ensure consistency with the validated field constraints.

### Canonical Valid AuditEvent (used in TestTS02_1 and TestTS02_3)

```json
{
  "id":         "7c10ec9b-daaf-4146-8e40-6efc92e5db39",
  "timestamp":  "2026-06-18T11:26:26.527713+00:00",
  "run_id":     "20260618_112626_17dd3f",
  "event_type": "run.start",
  "node_id":    "",
  "session_id": "",
  "archetype":  "",
  "severity":   "info",
  "payload":    {"plan_hash": "abc123"}
}
```

Note: `node_id`, `session_id`, and `archetype` are intentionally empty strings,
which is valid per the schema. Tests for existing spec 01 tests (`TestTS01_1`,
`TestTS01_2`, `TestTS01_5`) should also use this payload shape, substituting
distinct `id` values as needed.

### Invalid Payloads (used in TestTS02_2)

TestTS02_2 must exercise at least the following three invalid payload cases,
each submitted as a separate request, asserting `201 Created` and no DB row
for each:

| Case | Payload Description | Example |
|------|---------------------|---------|
| (a)  | Missing required field (`event_type` omitted) | All 9 fields except `event_type` |
| (b)  | `payload` is an array instead of an object | `"payload": ["not", "an", "object"]` |
| (c)  | `id` is an empty string | `"id": ""` with all other fields valid |

## File Change Manifest

The following source files must be created or modified as part of this spec.
No other files require changes.

| File                                            | Change Type | Description                                                                                  |
|-------------------------------------------------|-------------|----------------------------------------------------------------------------------------------|
| `internal/db/db.go`                             | Modify      | Update schema DDL to use `CREATE TABLE IF NOT EXISTS` with the new 10-column schema. Add fatal-exit behaviour if CREATE fails. Remove any DROP TABLE logic. |
| `internal/db/events.go`                         | Modify      | Add `AuditEvent` struct (with JSON tags). Update `InsertEvent` signature to accept `AuditEvent`. Implement `INSERT OR IGNORE` logic with bare `db.ExecContext`. |
| `internal/handler/events.go`                    | Modify      | Add validation logic: single-pass `json.Unmarshal` into `AuditEvent`, non-empty string checks, payload first-byte check (`'{'`), silently return 201 for failures, call `db.InsertEvent` on success via `h.DB`. |
| `internal/integration/events_test.go`           | Modify      | Update `TestTS01_1`, `TestTS01_2`, `TestTS01_5` to send the canonical valid `AuditEvent` JSON payload defined above.    |
| `internal/integration/validation_test.go`       | Create      | Add new tests `TestTS02_1`, `TestTS02_2`, `TestTS02_3` using the fixture payloads defined above. |
| `internal/integration/helpers_test.go`          | Modify      | Update test schema helper to match the new `events` table schema (10 columns).                            |
| `internal/db/db_test.go`                        | Modify      | Update unit tests to match the new `events` table schema and `InsertEvent` signature.        |

## Integration Test Updates (spec 01_basic_svc)

The following changes to the integration test suite from `01_basic_svc` are
required as part of this spec:

### Tests to Update

The following existing tests send arbitrary JSON payloads and must be updated
to send the canonical valid audit event JSON defined in the Integration Test
Fixtures section above:

| Test          | Required Change                                                  |
|---------------|------------------------------------------------------------------|
| `TestTS01_1`  | Replace arbitrary payload with the canonical valid `AuditEvent` JSON body.   |
| `TestTS01_2`  | Replace arbitrary payload with the canonical valid `AuditEvent` JSON body (distinct `id`).   |
| `TestTS01_5`  | Replace arbitrary payload with the canonical valid `AuditEvent` JSON body (distinct `id`).   |

Tests that cover 400/401/500 error conditions (authentication failures, missing
Content-Type, empty body, etc.) remain unchanged, as those code paths are
unaffected by this spec.

### New Tests to Add

| Test ID        | Scenario                                                                                          |
|----------------|---------------------------------------------------------------------------------------------------|
| `TestTS02_1`   | POST the canonical valid audit event; assert 201 with zero-byte body; query DB and verify all 9 event columns match the submitted event. Assert `received_at` is present and parseable as RFC3339Nano (do not assert an exact value). |
| `TestTS02_2`   | POST each of the three invalid payloads defined above (missing field, array payload, empty `id`); assert 201 with zero-byte body for each; verify no rows are stored in the DB. |
| `TestTS02_3`   | POST the canonical valid audit event twice (same `id`); assert 201 both times; verify only one row exists in the DB (duplicate silently ignored). |

## Design Decisions

1. **Silent rejection returns 201 Created** — returning the same response for
   valid and invalid events means attackers cannot distinguish accepted from
   rejected events, preventing schema probing and error-based DoS. The response
   is always a zero-byte body with no `Content-Type` header, implemented via
   `c.NoContent(http.StatusCreated)`.

2. **Event `id` is the primary key** — the audit events already carry their own
   identifier, and the instruction is to "match the structure exactly." Using
   the event's ID avoids generating redundant server-side UUIDs.

3. **`received_at` retained as additional column** — while not part of the event
   structure, a server-side timestamp is operationally useful for tracking
   ingestion latency. Stored as RFC3339Nano using a DATETIME column (stored as
   TEXT by SQLite's type affinity) to match existing behaviour.

4. **Duplicates silently ignored** — consistent with the "silent rejection"
   approach. If the same event arrives twice, the second is dropped without
   error via `INSERT OR IGNORE`, preventing replay-based storage exhaustion.

5. **Type-only validation, not format validation** — we check that `id` is a
   non-empty string but do not enforce UUID format. Similarly, `timestamp` must
   be a non-empty string but is not parsed as ISO 8601. This ensures forward
   compatibility if field formats evolve. The `timestamp` column is stored as
   TEXT for the same reason: avoiding parse failures if the timestamp format
   changes in future agent-fox versions.

6. **No event type enumeration** — `event_type` accepts any non-empty string.
   Hardcoding the current 12 types would require redeployment for each new type.

7. **No DROP TABLE on startup; CREATE TABLE IF NOT EXISTS only** — automatically
   dropping the table on every startup is destructive and unsafe in rolling
   restart or multi-instance scenarios. `CREATE TABLE IF NOT EXISTS` is safe and
   sufficient for a non-production service. Manual deletion of the database file
   is the supported migration path from the old schema. If `CREATE TABLE` fails,
   the service logs a fatal error and exits.

8. **No rejection observability in this spec** — emitting metrics for rejected
   events (e.g., a Prometheus counter) is explicitly deferred to a follow-up
   spec. The silent-rejection contract is preserved in full for this change.

9. **`InsertEvent` accepts a struct, not flat args** — using an `AuditEvent`
   struct as the parameter to `InsertEvent` keeps the function signature stable
   as the event schema evolves and avoids positional argument confusion across
   9 fields.

10. **`payload` is re-serialized on storage** — the `payload` field is stored
    as the output of `json.Marshal` on the parsed value rather than as raw
    request bytes. Downstream consumers are expected to parse stored JSON and
    must not rely on a specific byte representation (key order, whitespace).

11. **No body size limit in this spec** — Echo framework defaults apply. A
    configurable body size limit can be introduced in a follow-up spec if
    operational monitoring reveals abuse.

12. **`AuditEvent` struct stays in the `db` package** — the codebase is small
    and the struct is closely tied to the insert logic. No separate `audit` or
    `model` package is introduced. The struct's JSON tags allow it to be used
    directly by the handler for unmarshalling without a separate DTO.

13. **Non-empty but invalid JSON returns 201, not 400** — only an empty body
    returns 400 (preserved existing behaviour). Any non-empty body that fails
    JSON parsing or audit event validation is treated as an invalid event and
    silently rejected with 201, consistent with the schema-probing defence
    rationale.

14. **Single-pass unmarshal, no `DisallowUnknownFields`** — the handler performs
    a single `json.Unmarshal` into the `AuditEvent` struct. Unknown fields are
    silently ignored (Go default). `DisallowUnknownFields` is explicitly
    prohibited to ensure forward compatibility with new agent-fox event fields.

15. **Payload object check via first-byte inspection** — after unmarshalling,
    the `payload` field's `json.RawMessage` is checked by trimming whitespace
    and asserting the first byte is `'{'`. This is the simplest correct approach
    and avoids a secondary unmarshal pass.

16. **No explicit transaction for single-row insert** — `InsertEvent` uses a
    bare `db.ExecContext` call without wrapping in an explicit transaction.
    A single-row insert is atomic in SQLite by definition.

17. **Handler DB access via existing struct injection** — the handler accesses
    the `*sql.DB` handle via the existing `EventsHandler.DB` field. No new
    injection mechanism is introduced.

## Dependencies

| Spec | From Group | To Group | Relationship |
|------|-----------|----------|--------------|
| 01_basic_svc | 1 (tests) | 1 | Modifies integration tests first written in 01_basic_svc group 1 |

## Owner

agent-fox team (same owner as the parent spec `01_basic_svc`).

## Source

Source: `.agent-fox/audit/audit_20260618_112626_17dd3f.jsonl` — analyzed structure of 855 audit events containing 12 distinct event types across 9 top-level fields.
