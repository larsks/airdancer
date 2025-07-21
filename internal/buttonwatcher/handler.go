package buttonwatcher

import (
	"fmt"

	"github.com/larsks/airdancer/internal/cli"
)

// ButtonHandler implements cli.CommandHandler for the button watcher
type ButtonHandler struct{}

// NewButtonHandler creates a new button watcher command handler
func NewButtonHandler() *ButtonHandler {
	return &ButtonHandler{}
}

// Start starts the button watcher with the given configuration
func (h *ButtonHandler) Start(config cli.Configurable) error {
	cfg, ok := config.(*Config)
	if !ok {
		return fmt.Errorf("invalid config type for button watcher")
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	monitor := NewButtonMonitor()
	defer monitor.Close() //nolint:errcheck

	// Set global configuration for defaults
	monitor.SetGlobalConfig(cfg)

	for _, buttonCfg := range cfg.Buttons {
		if err := monitor.AddButtonFromConfig(buttonCfg); err != nil {
			return fmt.Errorf("failed to add button %s to monitor: %w", buttonCfg.Name, err)
		}
	}

	if err := monitor.Start(); err != nil {
		return fmt.Errorf("failed to start monitor: %w", err)
	}

	return nil
}
