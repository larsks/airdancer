package buttondriver

import (
	"fmt"
	"time"

	"github.com/larsks/airdancer/internal/buttondriver/common"
	"github.com/larsks/airdancer/internal/buttondriver/gpio"
	gpiotypes "github.com/larsks/airdancer/internal/gpio"
)

// GPIODriverConfig represents GPIO driver configuration
type GPIODriverConfig struct {
	PullMode   string `mapstructure:"pull-mode"`
	DebounceMs int    `mapstructure:"debounce-ms"`
}

// GPIODriverFactory implements Factory for GPIO button drivers
type GPIODriverFactory struct{}

// CreateDriver creates a new GPIO button driver
func (f *GPIODriverFactory) CreateDriver(config map[string]interface{}) (common.ButtonDriver, error) {
	cfg, err := f.parseConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse GPIO driver config: %w", err)
	}

	// Convert pull mode string to enum
	var pullModeEnum gpiotypes.PullMode
	switch cfg.PullMode {
	case "none":
		pullModeEnum = gpiotypes.PullNone
	case "up":
		pullModeEnum = gpiotypes.PullUp
	case "down":
		pullModeEnum = gpiotypes.PullDown
	case "auto":
		pullModeEnum = gpiotypes.PullAuto
	case "":
		pullModeEnum = gpiotypes.PullAuto // default
	default:
		return nil, fmt.Errorf("invalid pull mode: %s", cfg.PullMode)
	}

	debounceDelay := time.Duration(cfg.DebounceMs) * time.Millisecond
	return gpio.NewButtonDriver(debounceDelay, pullModeEnum)
}

// ParseButtonSpec parses a GPIO button specification string
func (f *GPIODriverFactory) ParseButtonSpec(spec string) (interface{}, error) {
	return gpio.ParseGPIOButtonSpec(spec)
}

// ValidateConfig validates GPIO driver configuration
func (f *GPIODriverFactory) ValidateConfig(config map[string]interface{}) error {
	cfg, err := f.parseConfig(config)
	if err != nil {
		return err
	}

	// Validate pull mode
	switch cfg.PullMode {
	case "", "none", "up", "down", "auto":
		// Valid values
	default:
		return fmt.Errorf("invalid pull mode: %s (must be one of: none, up, down, auto)", cfg.PullMode)
	}

	// Validate debounce time
	if cfg.DebounceMs < 0 {
		return fmt.Errorf("debounce-ms must be non-negative")
	}

	return nil
}

// parseConfig converts map to GPIODriverConfig struct
func (f *GPIODriverFactory) parseConfig(config map[string]interface{}) (*GPIODriverConfig, error) {
	cfg := &GPIODriverConfig{
		PullMode:   "auto", // default
		DebounceMs: 50,     // default 50ms
	}

	if pullMode, ok := config["pull-mode"].(string); ok {
		cfg.PullMode = pullMode
	}

	if debounceMs, ok := config["debounce-ms"].(int); ok {
		cfg.DebounceMs = debounceMs
	} else if debounceMs, ok := config["debounce-ms"].(float64); ok {
		cfg.DebounceMs = int(debounceMs)
	}

	return cfg, nil
}

func init() {
	MustRegister("gpio", &GPIODriverFactory{})
}
