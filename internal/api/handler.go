package api

import (
	"fmt"

	"github.com/larsks/airdancer/internal/cli"
)

// APIHandler implements cli.CommandHandler for the API server
type APIHandler struct{}

// NewAPIHandler creates a new API command handler
func NewAPIHandler() *APIHandler {
	return &APIHandler{}
}

// Start starts the API server with the given configuration
func (h *APIHandler) Start(config cli.Configurable) error {
	cfg, ok := config.(*Config)
	if !ok {
		return fmt.Errorf("invalid config type for API server")
	}

	srv, err := NewServer(cfg)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
	defer srv.Close() //nolint:errcheck

	if err := srv.Start(); err != nil {
		return fmt.Errorf("server failed: %w", err)
	}

	return nil
}
