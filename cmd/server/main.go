// Package main is the entrypoint for the basic_svc HTTP server.
package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/labstack/echo/v4"

	"github.com/agent-fox/example-service/internal/config"
	"github.com/agent-fox/example-service/internal/db"
	"github.com/agent-fox/example-service/internal/handler"
	"github.com/agent-fox/example-service/internal/logger"
	"github.com/agent-fox/example-service/internal/middleware"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "startup error: %v\n", err)
		os.Exit(1)
	}

	log, err := logger.New(cfg.LogLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "startup error: %v\n", err)
		os.Exit(1)
	}

	// Set as the default logger so any package-level slog calls also emit JSON.
	slog.SetDefault(log)

	log.Info("configuration loaded",
		slog.String("port", cfg.Port),
		slog.String("db_path", cfg.DBPath),
		slog.String("log_level", cfg.LogLevel),
	)

	// Open the SQLite database and create the schema.
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Error("failed to open database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer database.Close()

	// Build the Echo application.
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Add structured JSON request logging middleware.
	e.Use(logger.RequestLogger(log))

	// Register handlers.
	eventsHandler := &handler.EventsHandler{DB: database, Logger: log}
	livenessHandler := &handler.LivenessHandler{Logger: log}
	readinessHandler := &handler.ReadinessHandler{DB: database, Logger: log}

	e.POST("/v1/events", eventsHandler.Handle, middleware.BearerAuth(cfg.AuthBearerToken))
	e.GET("/healthz", livenessHandler.Handle)
	e.GET("/readyz", readinessHandler.Handle)

	// Start the HTTP server.
	// TODO: implement graceful shutdown in task group 9
	addr := ":" + cfg.Port
	log.Info("starting server", slog.String("addr", addr))
	if err := e.Start(addr); err != nil {
		log.Error("server error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
