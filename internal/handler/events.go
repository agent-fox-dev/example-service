// Package handler provides HTTP request handlers for the service.
package handler

import (
	"database/sql"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/agent-fox/example-service/internal/db"
)

// EventsHandler handles POST /v1/events requests.
type EventsHandler struct {
	DB     *sql.DB
	Logger *slog.Logger
}

// Handle processes an event ingestion request.
// It validates the Content-Type header and body, then stores the raw JSON
// payload in the events table.
func (h *EventsHandler) Handle(c echo.Context) error {
	// Validate Content-Type is application/json.
	ct := c.Request().Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		return echo.NewHTTPError(http.StatusBadRequest, "Content-Type must be application/json")
	}

	// Read and validate the request body is non-empty.
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to read request body")
	}
	if len(body) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "request body must not be empty")
	}

	// Store the raw JSON payload.
	if err := db.InsertEvent(c.Request().Context(), h.DB, string(body)); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to store event")
	}

	return c.NoContent(http.StatusCreated)
}
