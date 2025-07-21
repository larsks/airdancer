package switchdrivers

import (
	"fmt"

	"github.com/larsks/airdancer/internal/piface"
	"github.com/larsks/airdancer/internal/switchcollection"
)

// PiFaceConfig represents PiFace driver configuration
type PiFaceConfig struct {
	SPIDev      string `mapstructure:"spidev"`
	MaxSwitches uint   `mapstructure:"max-switches"`
}

// PiFaceFactory implements Factory for PiFace drivers
type PiFaceFactory struct{}

// CreateDriver creates a new PiFace switch collection
func (f *PiFaceFactory) CreateDriver(config map[string]interface{}) (switchcollection.SwitchCollection, error) {
	cfg, err := f.parseConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PiFace config: %w", err)
	}

	spidev := cfg.SPIDev
	if spidev == "" {
		spidev = "/dev/spidev0.0"
	}

	sc, err := piface.NewPiFace(true, spidev, cfg.MaxSwitches)
	if err != nil {
		return nil, fmt.Errorf("failed to create PiFace on %s: %w", spidev, err)
	}

	return sc, nil
}

// ValidateConfig validates PiFace configuration
func (f *PiFaceFactory) ValidateConfig(config map[string]interface{}) error {
	_, err := f.parseConfig(config)
	return err
}

// parseConfig converts map to PiFaceConfig struct
func (f *PiFaceFactory) parseConfig(config map[string]interface{}) (*PiFaceConfig, error) {
	cfg := &PiFaceConfig{}

	if spidev, ok := config["spidev"].(string); ok {
		cfg.SPIDev = spidev
	}

	if maxSwitches, ok := config["max-switches"].(uint); ok {
		cfg.MaxSwitches = maxSwitches
	} else if maxSwitches, ok := config["max-switches"].(int); ok {
		if maxSwitches < 0 {
			return nil, fmt.Errorf("max-switches must be non-negative")
		}
		cfg.MaxSwitches = uint(maxSwitches)
	}

	return cfg, nil
}

func init() {
	Register("piface", &PiFaceFactory{})
}
