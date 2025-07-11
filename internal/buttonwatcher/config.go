package buttonwatcher

import (
	"fmt"
	"time"

	"github.com/larsks/airdancer/internal/config"
	"github.com/spf13/pflag"
)

type ButtonConfig struct {
	Name               string         `mapstructure:"name"`
	Driver             string         `mapstructure:"driver"`
	Spec               string         `mapstructure:"spec"`
	ClickAction        *string        `mapstructure:"click_action"`
	DoubleClickAction  *string        `mapstructure:"double_click_action"`
	TripleClickAction  *string        `mapstructure:"triple_click_action"`
	ClickInterval      *time.Duration `mapstructure:"click_interval"`
	ShortPressDuration *time.Duration `mapstructure:"short_press_duration"`
	ShortPressAction   *string        `mapstructure:"short_press_action"`
	LongPressDuration  *time.Duration `mapstructure:"long_press_duration"`
	LongPressAction    *string        `mapstructure:"long_press_action"`
	Timeout            *time.Duration `mapstructure:"timeout"`
}

type Config struct {
	ConfigFile         string         `mapstructure:"config-file"`
	Buttons            []ButtonConfig `mapstructure:"buttons"`
	
	// Global defaults for timing-related settings
	ClickInterval      *time.Duration `mapstructure:"click_interval"`
	ShortPressDuration *time.Duration `mapstructure:"short_press_duration"`
	LongPressDuration  *time.Duration `mapstructure:"long_press_duration"`
	Timeout            *time.Duration `mapstructure:"timeout"`
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
		if button.Driver == "" {
			return fmt.Errorf("button %d (%s): driver is required", i, button.Name)
		}
		if button.Spec == "" {
			return fmt.Errorf("button %d (%s): spec is required", i, button.Name)
		}
	}
	return nil
}
