package buttondriver

import (
	"fmt"

	"github.com/larsks/airdancer/internal/buttondriver/common"
	"github.com/larsks/airdancer/internal/buttondriver/event"
)

// EventDriverConfig represents event driver configuration
type EventDriverConfig struct {
	// Event drivers typically don't need much configuration
	// This is here for consistency and future extensibility
}

// EventDriverFactory implements Factory for event button drivers
type EventDriverFactory struct{}

// CreateDriver creates a new event button driver
func (f *EventDriverFactory) CreateDriver(config map[string]interface{}) (common.ButtonDriver, error) {
	// Validate config if needed
	if err := f.ValidateConfig(config); err != nil {
		return nil, fmt.Errorf("failed to validate event driver config: %w", err)
	}

	return event.NewEventButtonDriver(), nil
}

// ParseButtonSpec parses an event button specification string
func (f *EventDriverFactory) ParseButtonSpec(spec string) (interface{}, error) {
	return event.ParseEventButtonSpec(spec)
}

// ValidateConfig validates event driver configuration
func (f *EventDriverFactory) ValidateConfig(config map[string]interface{}) error {
	// Event drivers don't currently have configuration parameters,
	// but this validates that no invalid parameters are provided
	validKeys := map[string]bool{
		// Add any future config keys here
	}

	for key := range config {
		if !validKeys[key] {
			return fmt.Errorf("unknown configuration parameter for event driver: %s", key)
		}
	}

	return nil
}

// parseConfig converts map to EventDriverConfig struct
func (f *EventDriverFactory) parseConfig(config map[string]interface{}) (*EventDriverConfig, error) {
	cfg := &EventDriverConfig{}
	// Add parsing logic here when event driver gets configuration parameters
	return cfg, nil
}

func init() {
	Register("event", &EventDriverFactory{})
}
