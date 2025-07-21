package monitor

import (
	"fmt"
	"log"

	"github.com/larsks/airdancer/internal/cli"
)

// MonitorHandler implements cli.CommandHandler for the email monitor
type MonitorHandler struct{}

// NewMonitorHandler creates a new monitor command handler
func NewMonitorHandler() *MonitorHandler {
	return &MonitorHandler{}
}

// Start starts the email monitor with the given configuration
func (h *MonitorHandler) Start(config cli.Configurable) error {
	cfg, ok := config.(*Config)
	if !ok {
		return fmt.Errorf("invalid config type for email monitor")
	}

	// Monitor has special config loading behavior - if config file is not explicitly set
	// and default doesn't exist, continue with defaults but log warning
	if cfg.ConfigFile == "" {
		log.Printf("using default configuration: no config file specified")
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	// Create email monitor
	emailMonitor, err := NewEmailMonitorWithDefaults(*cfg)
	if err != nil {
		return fmt.Errorf("failed to create monitor: %w", err)
	}

	// Start monitoring (this blocks)
	emailMonitor.Start()

	return nil
}
