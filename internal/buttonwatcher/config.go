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
	ClickAction        *string        `mapstructure:"click-action"`
	DoubleClickAction  *string        `mapstructure:"double-click-action"`
	TripleClickAction  *string        `mapstructure:"triple-click-action"`
	ClickInterval      *time.Duration `mapstructure:"click-interval"`
	ShortPressDuration *time.Duration `mapstructure:"short-press-duration"`
	ShortPressAction   *string        `mapstructure:"short-press-action"`
	LongPressDuration  *time.Duration `mapstructure:"long-press-duration"`
	LongPressAction    *string        `mapstructure:"long-press-action"`
	Timeout            *time.Duration `mapstructure:"timeout"`
	DefaultAction      *string        `mapstructure:"default-action"`
}

type Config struct {
	ConfigFile string         `mapstructure:"config-file"`
	Buttons    []ButtonConfig `mapstructure:"buttons"`

	// Global defaults for timing-related settings
	ClickInterval      *time.Duration `mapstructure:"click-interval"`
	ShortPressDuration *time.Duration `mapstructure:"short-press-duration"`
	LongPressDuration  *time.Duration `mapstructure:"long-press-duration"`
	Timeout            *time.Duration `mapstructure:"timeout"`
	DefaultAction      *string        `mapstructure:"default-action"`
}

func NewConfig() *Config {
	return &Config{}
}

func (c *Config) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&c.ConfigFile, "config", c.ConfigFile, "Config file to use")
}

func (c *Config) LoadConfig() error {
	return c.LoadConfigWithFlagSet(pflag.CommandLine)
}

func (c *Config) LoadConfigWithFlagSet(fs *pflag.FlagSet) error {
	loader := config.NewConfigLoader()
	loader.SetConfigFile(c.ConfigFile)
	return loader.LoadConfigWithFlagSet(c, fs)
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

		// Check that the button has at least one action configured
		hasAction := button.ClickAction != nil ||
			button.DoubleClickAction != nil ||
			button.TripleClickAction != nil ||
			button.ShortPressAction != nil ||
			button.LongPressAction != nil ||
			button.DefaultAction != nil ||
			c.DefaultAction != nil

		if !hasAction {
			return fmt.Errorf("button %d (%s): no actions configured (no global default-action or button-specific actions)", i, button.Name)
		}
	}
	return nil
}
