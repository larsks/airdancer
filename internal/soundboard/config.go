package soundboard

import (
	"strings"

	"github.com/larsks/airdancer/internal/config"
	"github.com/spf13/pflag"
)

// Config holds configuration for the soundboard service
type Config struct {
	// ConfigFile holds the path to the configuration file
	ConfigFile string `mapstructure:"config-file"`
	// ListenAddress is the address to bind the HTTP server to
	ListenAddress string `mapstructure:"listen-address"`
	// ListenPort is the port to bind the HTTP server to
	ListenPort int `mapstructure:"listen-port"`
	// SoundDirectory is the path to the directory containing sound files
	SoundDirectory string `mapstructure:"sound-directory"`
	// ItemsPerPage is the default number of items to show per page
	ItemsPerPage int `mapstructure:"items-per-page"`
	// BaseURL is the base URL path when hosted behind a proxy (e.g., "/soundboard")
	BaseURL string `mapstructure:"base-url"`
	// ALSADevice is the ALSA device to use for server-side audio playback
	ALSADevice string `mapstructure:"alsa-device"`
	// ALSACardName is the ALSA card name to use for server-side audio playback
	ALSACardName string `mapstructure:"alsa-card-name"`
	// ScanInterval is the interval in seconds to scan for sound directory changes (0 = disabled)
	ScanInterval int `mapstructure:"scan-interval"`
}

// NewConfig creates a new Config with default values
func NewConfig() *Config {
	return &Config{
		ListenAddress:  "",
		ListenPort:     8082,
		SoundDirectory: "./sounds",
		ItemsPerPage:   20,
		BaseURL:        "",
		ALSADevice:     "default",
		ALSACardName:   "",
		ScanInterval:   30, // Default to 30 seconds
	}
}

// AddFlags adds command line flags for this config
func (c *Config) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&c.ConfigFile, "config", "", "Path to configuration file")
	fs.StringVar(&c.ListenAddress, "listen-address", c.ListenAddress, "Address to bind HTTP server to")
	fs.IntVar(&c.ListenPort, "listen-port", c.ListenPort, "Port to bind HTTP server to")
	fs.StringVar(&c.SoundDirectory, "sound-directory", c.SoundDirectory, "Directory containing sound files")
	fs.IntVar(&c.ItemsPerPage, "items-per-page", c.ItemsPerPage, "Default number of items per page")
	fs.StringVar(&c.BaseURL, "base-url", c.BaseURL, "Base URL path when hosted behind a proxy (e.g., '/soundboard')")
	fs.StringVar(&c.ALSADevice, "alsa-device", c.ALSADevice, "ALSA device for server-side audio playback")
	fs.StringVar(&c.ALSACardName, "alsa-card-name", c.ALSACardName, "ALSA card name for server-side audio playback")
	fs.IntVar(&c.ScanInterval, "scan-interval", c.ScanInterval, "Interval in seconds to scan for sound directory changes (0 = disabled)")
}

// LoadConfig loads configuration using the standard config pattern
func (c *Config) LoadConfig() error {
	defaults := map[string]any{
		"listen-address":  "",
		"listen-port":     8082,
		"sound-directory": "./sounds",
		"items-per-page":  20,
		"base-url":        "",
		"alsa-device":     "default",
		"alsa-card-name":  "",
		"scan-interval":   30,
	}

	return config.StandardConfigPattern(c, c.ConfigFile, defaults)
}

// LoadConfigWithFlagSet loads configuration using a custom flag set
func (c *Config) LoadConfigWithFlagSet(fs *pflag.FlagSet) error {
	loader := config.NewConfigLoader()
	loader.SetConfigFile(c.ConfigFile)
	loader.SetDefaults(map[string]any{
		"listen-address":  "",
		"listen-port":     8082,
		"sound-directory": "./sounds",
		"items-per-page":  20,
		"base-url":        "",
		"alsa-device":     "default",
		"alsa-card-name":  "",
		"scan-interval":   30,
	})

	return loader.LoadConfigWithFlagSet(c, fs)
}

// GetBaseURL returns the normalized base URL path
func (c *Config) GetBaseURL() string {
	if c.BaseURL == "" {
		return ""
	}
	
	baseURL := strings.TrimSpace(c.BaseURL)
	
	// Ensure it starts with /
	if !strings.HasPrefix(baseURL, "/") {
		baseURL = "/" + baseURL
	}
	
	// Ensure it doesn't end with /
	baseURL = strings.TrimSuffix(baseURL, "/")
	
	return baseURL
}

// GetFullPath returns a full path including the base URL
func (c *Config) GetFullPath(path string) string {
	baseURL := c.GetBaseURL()
	if baseURL == "" {
		return path
	}
	
	// Ensure path starts with /
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	
	return baseURL + path
}
