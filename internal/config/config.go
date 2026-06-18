// Package config provides configuration loading from environment variables.
package config

import (
	"fmt"
	"os"
)

// Config holds the application configuration.
type Config struct {
	Port            string
	DBPath          string
	AuthBearerToken string
	LogLevel        string
}

// validLogLevels is the set of accepted LOG_LEVEL values.
var validLogLevels = map[string]bool{
	"debug": true,
	"info":  true,
	"warn":  true,
	"error": true,
}

// Load reads configuration from environment variables.
// Returns an error if required variables are missing or invalid.
//
// Environment variables:
//   - PORT: HTTP listen port (default "8080")
//   - DB_PATH: SQLite database file path (default "./data/events.db")
//   - AUTH_BEARER_TOKEN: static bearer token for authentication (required, no default)
//   - LOG_LEVEL: logging verbosity — debug, info, warn, or error (default "info")
func Load() (Config, error) {
	cfg := Config{
		Port:     envOrDefault("PORT", "8080"),
		DBPath:   envOrDefault("DB_PATH", "./data/events.db"),
		LogLevel: envOrDefault("LOG_LEVEL", "info"),
	}

	cfg.AuthBearerToken = os.Getenv("AUTH_BEARER_TOKEN")
	if cfg.AuthBearerToken == "" {
		return Config{}, fmt.Errorf("AUTH_BEARER_TOKEN environment variable is required")
	}

	if !validLogLevels[cfg.LogLevel] {
		return Config{}, fmt.Errorf("LOG_LEVEL must be one of: debug, info, warn, error; got %q", cfg.LogLevel)
	}

	return cfg, nil
}

// envOrDefault returns the value of the named environment variable, or
// fallback if the variable is empty or unset.
func envOrDefault(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}
