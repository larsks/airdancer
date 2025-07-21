package soundboard

import (
	"fmt"

	"github.com/larsks/airdancer/internal/cli"
)

// SoundboardHandler implements cli.CommandHandler for the soundboard server
type SoundboardHandler struct{}

// NewSoundboardHandler creates a new soundboard command handler
func NewSoundboardHandler() *SoundboardHandler {
	return &SoundboardHandler{}
}

// Start starts the soundboard server with the given configuration
func (h *SoundboardHandler) Start(config cli.Configurable) error {
	cfg, ok := config.(*Config)
	if !ok {
		return fmt.Errorf("invalid config type for soundboard server")
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
