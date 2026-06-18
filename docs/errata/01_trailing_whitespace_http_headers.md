# Erratum: Trailing Whitespace in HTTP Header Values (Spec 01)

## Affected Test

- `TestTS01_P2_UnauthenticatedRequestsNeverReachStorage/trailing_space_token`
  in `internal/integration/property_test.go`

## Property

01-PROP-2 (Unauthenticated requests never reach storage)

## Issue

The test case `trailing_space_token` sends an Authorization header with a
trailing space: `"Bearer test-secret "`. It expects the middleware to reject
this as an invalid token (HTTP 401).

However, Go's `net/textproto` reader (used by `net/http`) strips trailing
whitespace from HTTP header values during parsing, per RFC 7230 Section 3.2.6
which states:

> A field value does not include leading or trailing whitespace.

By the time the header value reaches any handler or middleware code via
`r.Header.Get("Authorization")`, the trailing space has already been removed.
The middleware sees `"Bearer test-secret"` which is a valid, matching token.

## Evidence

```go
// Sending: "Bearer test-secret " (19 bytes)
// Received by handler: "Bearer test-secret" (18 bytes)
```

This is standard Go HTTP library behavior and cannot be overridden at the
middleware level without bypassing `net/http` entirely.

## Impact

The `trailing_space_token` sub-test within `TestTS01_P2` will always fail
with status 200 (or 201 once the handler is implemented) instead of the
expected 401. This is a single sub-test out of 14 property test variants.

All other authentication rejection scenarios work correctly.

## Resolution

This test case represents a spec assumption that conflicts with Go's HTTP
implementation of RFC 7230. The trailing whitespace is not observable by
application code because the HTTP transport layer removes it before delivery.

No middleware-level fix is possible. The test should be updated to remove this
case or accept the pass-through behavior as correct.
