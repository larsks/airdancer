package ui

import (
	"fmt"

	"github.com/larsks/airdancer/internal/cli"
	"github.com/larsks/airdancer/internal/httpserver"
)

// UIHandler implements cli.CommandHandler for the UI server
type UIHandler struct{}

// NewUIHandler creates a new UI command handler
func NewUIHandler() *UIHandler {
	return &UIHandler{}
}

// Start starts the UI server with the given configuration
func (h *UIHandler) Start(config cli.Configurable) error {
	cfg, ok := config.(*Config)
	if !ok {
		return fmt.Errorf("invalid config type for UI server")
	}

	srv := NewUIServer(cfg)
	return httpserver.StartFromConfig(cfg, srv)
}

// GetListenAddress implements httpserver.Config interface
func (c *Config) GetListenAddress() string {
	return c.ListenAddress
}

// GetListenPort implements httpserver.Config interface
func (c *Config) GetListenPort() int {
	return c.ListenPort
}
