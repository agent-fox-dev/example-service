# The canonical PRD

## Overview

A simple test service for validating tool spec and agent-fox (af) integration. The service receives audit events from agent-fox instances via HTTP POST and stores them in a local embedded SQLite database, providing a minimal, verifiable ingestion pipeline.

This service is a lightweight test harness for agent-fox (af) tooling and spec validation. No additional background context or prior art is defined at this stage. The service is designed for single-instance deployment on Kubernetes.

Running it locally can be done using Podman or directly via `go run`.

No specific measurable goals (latency, throughput, uptime, retention) are defined at this stage. This is an early-phase test service; success is defined by correct end-to-end operation of event ingestion and storage.


The following are explicitly out of scope for this version:

- Event retrieval or query API — there is no endpoint for reading, searching, or exporting stored audit events.
- Multi-instance or distributed deployment support.
- Alerting or event forwarding.
- Advanced storage strategies or data retention policies.

## Functional Requirements

- **Method & Path:** `POST /events` (or equivalent path to be confirmed during implementation)
- **Authentication:** The endpoint is secured by bearer token authentication. Requests without a valid bearer token must be rejected.
- **Payload:** The structure of the audit event payload is not formally specified at this stage. The handler should accept and store the raw JSON body as received.
- **Behaviour:** On receipt of a valid, authenticated request, the service stores the event in the local SQLite database and returns an appropriate HTTP success response.
- **Error Handling:** Standard HTTP error responses must be returned for invalid requests (e.g. `400 Bad Request`) and unauthorised requests (e.g. `401 Unauthorized`). Internal errors must return `500 Internal Server Error`. Error responses should follow the Echo Framework's default error handling conventions.

Standard Kubernetes health check endpoints must be implemented:

- `GET /healthz` — liveness probe; returns `200 OK` when the service process is running.
- `GET /readyz` — readiness probe; returns `200 OK` when the service is ready to accept traffic (e.g. database connection is available).

## Technical Specification

Standard Go project layout:

```
cmd/        # entrypoints
internal/   # packages not intended for external import
<module>/   # reusable packages that may be imported by other projects
```

### Tech Stack

- **Language:** Go
- **HTTP Framework:** Echo Framework
- **Database:** SQLite (embedded, via an appropriate Go SQLite driver)
- **Middleware:** Standard Echo Framework middleware for logging, error handling, and security

### Configuration

The service must be configurable via environment variables. At minimum, the following must be supported:

- HTTP listen port (default: `8080`)
- SQLite database file path (default: `./data/events.db`)
- Bearer token value used for endpoint authentication
