---
spec_id: '01'
spec_name: basic_svc
title: Basic Svc
status: draft
created_at: '2026-06-18T08:50:14.204993+00:00'
updated_at: '2026-06-18T08:50:14.204993+00:00'
owner: ''
source: interactive
schema_version: 1
---
# The canonical PRD

## Intent

To provide a minimal, verifiable HTTP ingestion service that receives and persists audit events from agent-fox instances for integration testing purposes.

## Background

This service is a lightweight test harness for agent-fox (af) tooling and spec validation. Running the same sequence of `af` and `spec` commands against the same `prd.md` file allows evaluation of the performance and correctness of `af` and `spec` commands. The service exists to provide a concrete, end-to-end verifiable integration target — receiving audit events over HTTP and persisting them to a local embedded database — so that agent-fox tooling can be exercised against a real, observable pipeline.

No prior approaches or alternative implementations are documented at this stage. The service is designed for single-instance deployment on Kubernetes and can also be run locally using Podman or directly via `go run`.

## Goals

- The service successfully accepts a `POST /v1/events` request with a valid bearer token and stores the raw event body in SQLite, returning a `201 Created` response. This constitutes the primary verifiable success criterion for this version.

## Non-Goals

The following are explicitly out of scope for this version:

- Event retrieval or query API — there is no endpoint for reading, searching, or exporting stored audit events.
- Multi-instance or distributed deployment support.
- Alerting or event forwarding.
- Advanced storage strategies or data retention policies.

## Overview

A simple test service for validating tool spec and agent-fox (af) integration. The service receives audit events from agent-fox instances via HTTP POST and stores them in a local embedded SQLite database, providing a minimal, verifiable ingestion pipeline.

## Functional Requirements

### Event Ingestion Endpoint

- **Method & Path:** `POST /v1/events`
- **Authentication:** The endpoint is secured by bearer token authentication. The token must be extracted from the `Authorization` header using exactly the format `Bearer <token>` — the prefix `Bearer ` is mandatory (capital `B`, single space). Any deviation, including a missing prefix, incorrect casing, extra whitespace, or a malformed header, must be rejected with `401 Unauthorized`. The bearer token value is compared against the static shared secret configured via the `AUTH_BEARER_TOKEN` environment variable.
- **Payload Validation:** Before storage, the service must validate that:
  - The request includes a `Content-Type: application/json` header; reject with `400 Bad Request` otherwise.
  - The request body is non-empty; reject with `400 Bad Request` if the body is empty.
- **Payload:** The structure of the audit event payload is not formally specified. The handler must accept and store the raw JSON body as received, without further schema validation.
- **Behaviour:** On receipt of a valid, authenticated request with a conforming payload, the service stores the event in the local SQLite database and returns `201 Created` with an empty response body. No `Content-Type` header is set on the `201 Created` response.
- **Error Handling:** Standard HTTP error responses must be returned:
  - `400 Bad Request` — missing or non-JSON `Content-Type`, or empty body.
  - `401 Unauthorized` — missing or invalid bearer token, or malformed `Authorization` header.
  - `500 Internal Server Error` — internal or database errors.
  - All error responses use Echo Framework's default error format: `{"message": "<error description>"}` with `Content-Type: application/json`.

### Health Check Endpoints

Standard Kubernetes health check endpoints must be implemented:

- `GET /healthz` — liveness probe; returns `200 OK` when the service process is running.
- `GET /readyz` — readiness probe; verifies database availability by executing a lightweight SQL ping (`SELECT 1`). Returns `200 OK` when the query succeeds. Returns `503 Service Unavailable` with body `{"message": "service unavailable"}` if the query fails or the database connection is otherwise unavailable.

## Technical Specification

### Go Module

The Go module path for this service is `github.com/agent-fox/example-service`. This must be used in `go.mod` and all internal import paths.

Standard Go project layout:

```
cmd/        # entrypoints
internal/   # packages not intended for external import
```

### Tech Stack

- **Language:** Go
- **HTTP Framework:** Echo Framework
- **Database:** SQLite (embedded), using the `modernc.org/sqlite` pure-Go driver. CGO is not required or used; this simplifies cross-compilation and container builds.
- **Middleware:** Standard Echo Framework middleware for logging, error handling, and security

### Database Schema

A single table named `events` must be created with the following schema:

| Column        | Type      | Constraints                          |
|---------------|-----------|--------------------------------------|
| `id`          | TEXT      | Primary key, UUID, generated on insert |
| `payload`     | TEXT      | Raw JSON body as received; NOT NULL  |
| `received_at` | DATETIME  | Auto-set to current UTC time on insert; NOT NULL |

No additional indexes are required at this stage.

### Configuration

The service must be configurable via environment variables. At minimum, the following must be supported:

| Variable             | Description                                      | Default              |
|----------------------|--------------------------------------------------|----------------------|
| `PORT`               | HTTP listen port                                 | `8080`               |
| `DB_PATH`            | SQLite database file path                        | `./data/events.db`   |
| `AUTH_BEARER_TOKEN`  | Static shared secret used for bearer token auth  | *(required, no default)* |
| `LOG_LEVEL`          | Logging verbosity level (e.g. `debug`, `info`, `warn`, `error`) | `info` |

The `AUTH_BEARER_TOKEN` variable has no default and must be explicitly set; the service must fail to start if it is absent.

### Logging

The service must emit structured JSON log output. All log entries must be formatted as JSON objects to support log aggregation pipelines in Kubernetes environments (e.g. Fluentd, Loki). The log verbosity level must be runtime-configurable via the `LOG_LEVEL` environment variable, accepting the standard levels `debug`, `info`, `warn`, and `error`. The default log level is `info`. Invalid values for `LOG_LEVEL` should cause the service to fail to start with a descriptive error message.

### Graceful Shutdown

The service must handle `SIGTERM` (and `SIGINT`) gracefully. On receiving the signal, the service must stop accepting new connections and allow in-flight requests to complete before exiting. A fixed shutdown timeout of **30 seconds** must be enforced — if in-flight requests have not completed within this window, the service exits regardless. This behaviour is required for safe Kubernetes pod termination.

### Testing

The service must include integration tests covering the full HTTP-to-SQLite pipeline. At minimum, integration tests must exercise:

- Successful event ingestion (`POST /v1/events` with valid token and payload → `201 Created`, event persisted in SQLite).
- Rejection of requests with missing or invalid bearer tokens → `401 Unauthorized`.
- Rejection of requests with missing `Content-Type: application/json` or empty body → `400 Bad Request`.
- Health check endpoints (`GET /healthz` and `GET /readyz`) under normal conditions.

Unit tests may be added at the implementer's discretion but are not mandated. No minimum coverage percentage is enforced.

## Owner

agent-fox team
