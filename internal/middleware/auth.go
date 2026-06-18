// Package middleware provides Echo middleware for the service.
package middleware

import "github.com/labstack/echo/v4"

// BearerAuth returns an Echo middleware that validates bearer token authentication.
func BearerAuth(token string) echo.MiddlewareFunc {
	// TODO: implement in task group 6
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			return next(c)
		}
	}
}
