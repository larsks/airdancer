package drivers

import (
	"fmt"

	"github.com/larsks/airdancer/internal/switchcollection"
	gpio "github.com/larsks/airdancer/internal/switchcollection/gpio_warthog"
)

// GPIOConfig represents GPIO driver configuration
type GPIOConfig struct {
	Pins []string `mapstructure:"pins"`
}

// GPIOFactory implements Factory for GPIO drivers
type GPIOFactory struct{}

// Create creates a new GPIO switch collection
func (f *GPIOFactory) Create(config map[string]interface{}) (switchcollection.SwitchCollection, error) {
	cfg, err := f.parseConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse GPIO config: %w", err)
	}

	if len(cfg.Pins) == 0 {
		return nil, fmt.Errorf("GPIO driver requires at least one pin")
	}

	sc, err := gpio.NewGPIOSwitchCollection(true, cfg.Pins)
	if err != nil {
		return nil, fmt.Errorf("failed to create GPIO driver with pins %v: %w", cfg.Pins, err)
	}

	return sc, nil
}

// ValidateConfig validates GPIO configuration
func (f *GPIOFactory) ValidateConfig(config map[string]interface{}) error {
	cfg, err := f.parseConfig(config)
	if err != nil {
		return err
	}

	if len(cfg.Pins) == 0 {
		return fmt.Errorf("GPIO driver requires at least one pin")
	}

	return nil
}

// parseConfig converts map to GPIOConfig struct
func (f *GPIOFactory) parseConfig(config map[string]interface{}) (*GPIOConfig, error) {
	cfg := &GPIOConfig{}

	if pins, ok := config["pins"].([]interface{}); ok {
		cfg.Pins = make([]string, len(pins))
		for i, pin := range pins {
			if pinStr, ok := pin.(string); ok {
				cfg.Pins[i] = pinStr
			} else {
				return nil, fmt.Errorf("pin %d is not a string", i)
			}
		}
	} else if pins, ok := config["pins"].([]string); ok {
		cfg.Pins = pins
	} else {
		return nil, fmt.Errorf("pins configuration is required and must be a string array")
	}

	return cfg, nil
}

func init() {
	Register("gpio", &GPIOFactory{})
}
