// Package main is the entrypoint for the basic_svc HTTP server.
package main

import (
	"fmt"
	"os"

	"github.com/agent-fox/example-service/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "startup error: %v\n", err)
		os.Exit(1)
	}

	// TODO: implement remaining startup in task groups 4, 5, 7, 8, 9
	_ = cfg
}
