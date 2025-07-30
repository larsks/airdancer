package status

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/adrg/xdg"
	"github.com/larsks/airdancer/internal/config"
	"github.com/spf13/pflag"
)

const defaultServerURL = "http://localhost:8080"

// Config holds the airdancer-status configuration
type Config struct {
	ServerURL          string        `mapstructure:"server-url"`
	UpdateInterval     time.Duration `mapstructure:"update-interval"`
	ConfigFile         string        `mapstructure:"config-file"`
	DryRun             bool          `mapstructure:"dry-run"`
	explicitConfigFile bool          // Track if config file was explicitly set
}

func getDefaultServerURL() string {
	if url := os.Getenv("DANCER_SERVER_URL"); url != "" {
		return url
	}

	return defaultServerURL
}

func getDefaultConfigFile() string {
	return filepath.Join(xdg.ConfigHome, "dancer", "dancer.toml")
}

// NewConfig creates a new Config with default values
func NewConfig() *Config {
	return &Config{
		ServerURL:      getDefaultServerURL(),
		UpdateInterval: 5 * time.Second,
	}
}

// AddFlags adds command-line flags for all configuration options
func (c *Config) AddFlags(fs *pflag.FlagSet) {
	defaultConfigFile := getDefaultConfigFile()
	fs.StringVar(&c.ConfigFile, "config", defaultConfigFile, "Config file to use")
	fs.StringVar(&c.ServerURL, "server-url", c.ServerURL, "API server URL")
	fs.DurationVarP(&c.UpdateInterval, "update-interval", "i", c.UpdateInterval, "Update interval for status loop")
	fs.BoolVarP(&c.DryRun, "dry-run", "n", c.DryRun, "Use fake display driver instead of hardware")
}

// LoadConfigFromStruct loads configuration with proper precedence using the common pattern
func (c *Config) LoadConfigFromStruct() error {
	return c.LoadConfigWithFlagSet(pflag.CommandLine)
}

// LoadConfigWithFlagSet loads configuration with proper precedence using a custom flag set (for testing)
func (c *Config) LoadConfigWithFlagSet(fs *pflag.FlagSet) error {
	// Check if config file was explicitly set by comparing with default
	defaultConfigFile := getDefaultConfigFile()
	c.explicitConfigFile = c.ConfigFile != defaultConfigFile

	loader := config.NewConfigLoader()

	// If using default config file, check if it exists and only set if it does
	if !c.explicitConfigFile {
		if _, err := os.Stat(c.ConfigFile); os.IsNotExist(err) {
			// Default config file doesn't exist, don't try to load it
			c.ConfigFile = ""
		}
	} else {
		// Explicit config file was specified, check if it exists
		if _, err := os.Stat(c.ConfigFile); os.IsNotExist(err) {
			return fmt.Errorf("config file not found: %s", c.ConfigFile)
		}
	}

	loader.SetConfigFile(c.ConfigFile)

	loader.SetDefaults(map[string]any{
		"server-url":      getDefaultServerURL(),
		"update-interval": 5 * time.Second,
		"dry-run":         false,
	})

	return loader.LoadConfigWithFlagSet(c, fs)
}
