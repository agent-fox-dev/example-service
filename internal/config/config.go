// Package config provides configuration loading from environment variables.
package config

// Config holds the application configuration.
type Config struct {
	Port            string
	DBPath          string
	AuthBearerToken string
	LogLevel        string
}

// Load reads configuration from environment variables.
// Returns an error if required variables are missing or invalid.
func Load() (Config, error) {
	// TODO: implement in task group 3
	return Config{}, nil
}
