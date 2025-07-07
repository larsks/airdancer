package buttonwatcher

import (
	"fmt"
	"time"

	"github.com/larsks/airdancer/internal/config"
	"github.com/spf13/pflag"
)

type ButtonConfig struct {
	Name               string         `mapstructure:"name"`
	Device             string         `mapstructure:"device"`
	EventType          string         `mapstructure:"event_type"`
	EventCode          uint32         `mapstructure:"event_code"`
	LowValue           *uint32        `mapstructure:"low_value"`
	HighValue          *uint32        `mapstructure:"high_value"`
	ClickAction        *string        `mapstructure:"click_action"`
	ShortPressDuration *time.Duration `mapstructure:"short_press_duration"`
	ShortPressAction   *string        `mapstructure:"short_press_action"`
	LongPressDuration  *time.Duration `mapstructure:"long_press_duration"`
	LongPressAction    *string        `mapstructure:"long_press_action"`
	Timeout            *time.Duration `mapstructure:"timeout"`
}

type Config struct {
	ConfigFile string         `mapstructure:"config-file"`
	Buttons    []ButtonConfig `mapstructure:"buttons"`
}

func NewConfig() *Config {
	return &Config{}
}

func (c *Config) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&c.ConfigFile, "config", c.ConfigFile, "Config file to use")
}

func (c *Config) LoadConfig() error {
	loader := config.NewConfigLoader()
	loader.SetConfigFile(c.ConfigFile)
	return loader.LoadConfig(c)
}

func (c *Config) Validate() error {
	if len(c.Buttons) == 0 {
		return fmt.Errorf("no buttons configured")
	}
	for i, button := range c.Buttons {
		if button.Name == "" {
			return fmt.Errorf("button %d: name is required", i)
		}
		if button.Device == "" {
			return fmt.Errorf("button %d (%s): device is required", i, button.Name)
		}
	}
	return nil
}
