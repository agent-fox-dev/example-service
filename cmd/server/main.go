// Package main is the entrypoint for the basic_svc HTTP server.
package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/agent-fox/example-service/internal/config"
	"github.com/agent-fox/example-service/internal/logger"
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

	// TODO: implement remaining startup in task groups 5, 7, 8, 9
	_ = cfg
}
