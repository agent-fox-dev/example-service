// Package main is the entrypoint for the basic_svc HTTP server.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	defer func() { _ = database.Close() }()

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

	// Start the HTTP server in a goroutine so main can wait for shutdown signals.
	addr := ":" + cfg.Port
	go func() {
		log.Info("starting server", slog.String("addr", addr))
		if err := e.Start(addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	// Wait for SIGTERM or SIGINT to initiate graceful shutdown.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	sig := <-quit
	log.Info("received shutdown signal", slog.String("signal", sig.String()))

	// Enforce a maximum shutdown timeout of 30 seconds. e.Shutdown stops
	// accepting new connections and waits for in-flight requests to complete.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := e.Shutdown(ctx); err != nil {
		log.Error("graceful shutdown timed out, forcing exit", slog.String("error", err.Error()))
		os.Exit(1)
	}

	log.Info("server stopped gracefully")
}
