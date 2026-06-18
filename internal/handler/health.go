package handler

import (
	"database/sql"
	"log/slog"

	"github.com/labstack/echo/v4"
)

// LivenessHandler handles GET /healthz requests.
type LivenessHandler struct {
	Logger *slog.Logger
}

// Handle returns 200 OK for liveness probes.
func (h *LivenessHandler) Handle(c echo.Context) error {
	// TODO: implement in task group 8
	return nil
}

// ReadinessHandler handles GET /readyz requests.
type ReadinessHandler struct {
	DB     *sql.DB
	Logger *slog.Logger
}

// Handle checks database connectivity and returns 200 OK or 503 Service Unavailable.
func (h *ReadinessHandler) Handle(c echo.Context) error {
	// TODO: implement in task group 8
	return nil
}
