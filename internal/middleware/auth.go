// Package middleware provides Echo middleware for the service.
package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

const bearerPrefix = "Bearer "

// BearerAuth returns an Echo middleware that validates bearer token authentication.
// It extracts the token from the Authorization header using exactly the format
// 'Bearer <token>' (capital B, single space) and compares it against the
// configured token value.
func BearerAuth(token string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authHeader := c.Request().Header.Get("Authorization")

			// Reject if the Authorization header is missing.
			if authHeader == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "missing Authorization header")
			}

			// Reject if the header does not begin with exactly "Bearer "
			// (capital B, single space). This catches wrong casing, extra
			// whitespace, missing prefix, and non-Bearer schemes.
			if !strings.HasPrefix(authHeader, bearerPrefix) {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid Authorization header format")
			}

			// Extract the token substring after the "Bearer " prefix.
			extractedToken := authHeader[len(bearerPrefix):]

			// Compare using constant-time comparison to prevent timing attacks.
			if subtle.ConstantTimeCompare([]byte(extractedToken), []byte(token)) != 1 {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid bearer token")
			}

			return next(c)
		}
	}
}
