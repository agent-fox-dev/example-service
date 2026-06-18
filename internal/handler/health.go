package handler

import (
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/agent-fox/example-service/internal/db"
)

// LivenessHandler handles GET /healthz requests.
type LivenessHandler struct {
	Logger *slog.Logger
}

// Handle returns 200 OK for liveness probes.
// It performs no database interaction — a successful response simply indicates
// the process is alive and accepting connections.
func (h *LivenessHandler) Handle(c echo.Context) error {
	return c.NoContent(http.StatusOK)
}

// ReadinessHandler handles GET /readyz requests.
type ReadinessHandler struct {
	DB     *sql.DB
	Logger *slog.Logger
}

// Handle checks database connectivity by executing a SELECT 1 query.
// Returns 200 OK when the database is available, or 503 Service Unavailable
// with body {"message": "service unavailable"} when the ping fails.
func (h *ReadinessHandler) Handle(c echo.Context) error {
	if err := db.Ping(c.Request().Context(), h.DB); err != nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"message": "service unavailable",
		})
	}
	return c.NoContent(http.StatusOK)
}
