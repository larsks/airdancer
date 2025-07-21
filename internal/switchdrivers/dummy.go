package switchdrivers

import (
	"fmt"

	"github.com/larsks/airdancer/internal/switchcollection"
)

// DummyConfig represents dummy driver configuration
type DummyConfig struct {
	SwitchCount uint `mapstructure:"switch-count"`
}

// DummyFactory implements Factory for dummy drivers
type DummyFactory struct{}

// CreateDriver creates a new dummy switch collection
func (f *DummyFactory) CreateDriver(config map[string]interface{}) (switchcollection.SwitchCollection, error) {
	cfg, err := f.parseConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse dummy config: %w", err)
	}

	if cfg.SwitchCount == 0 {
		cfg.SwitchCount = 4 // Default value
	}

	return switchcollection.NewDummySwitchCollection(cfg.SwitchCount), nil
}

// ValidateConfig validates dummy configuration
func (f *DummyFactory) ValidateConfig(config map[string]interface{}) error {
	_, err := f.parseConfig(config)
	return err
}

// parseConfig converts map to DummyConfig struct
func (f *DummyFactory) parseConfig(config map[string]interface{}) (*DummyConfig, error) {
	cfg := &DummyConfig{}

	if switchCount, ok := config["switch-count"].(uint); ok {
		cfg.SwitchCount = switchCount
	} else if switchCount, ok := config["switch-count"].(int); ok {
		if switchCount < 0 {
			return nil, fmt.Errorf("switch-count must be non-negative")
		}
		cfg.SwitchCount = uint(switchCount)
	}

	return cfg, nil
}

func init() {
	Register("dummy", &DummyFactory{})
}
