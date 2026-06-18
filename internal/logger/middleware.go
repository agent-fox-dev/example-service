package logger

import (
	"log/slog"
	"time"

	"github.com/labstack/echo/v4"
)

// RequestLogger returns Echo middleware that logs each HTTP request as a
// structured JSON entry via the provided slog.Logger. Each log line includes
// time (from slog), level, method, path, status, and latency fields.
func RequestLogger(log *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			err := next(c)
			if err != nil {
				c.Error(err)
			}

			req := c.Request()
			res := c.Response()
			latency := time.Since(start)

			log.Info("request",
				slog.String("method", req.Method),
				slog.String("path", req.URL.Path),
				slog.Int("status", res.Status),
				slog.String("latency", latency.String()),
			)

			return nil
		}
	}
}
