package config

import (
	"os"
	"testing"

	"github.com/spf13/pflag"
)

// TestConfig is a sample config struct for testing
type TestConfig struct {
	ConfigFile    string `mapstructure:"config-file"`
	ListenAddress string `mapstructure:"listen-address"`
	ListenPort    int    `mapstructure:"listen-port"`
	Debug         bool   `mapstructure:"debug"`
}

func (c *TestConfig) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&c.ConfigFile, "config", c.ConfigFile, "Config file to use")
	fs.StringVar(&c.ListenAddress, "listen-address", c.ListenAddress, "Listen address")
	fs.IntVar(&c.ListenPort, "listen-port", c.ListenPort, "Listen port")
	fs.BoolVar(&c.Debug, "debug", c.Debug, "Enable debug mode")
}

func TestConfigLoader_LoadConfig(t *testing.T) {
	// Create a temporary config file
	configContent := `
listen-address = "192.168.1.100"
listen-port = 9090
debug = true
`
	tmpFile, err := os.CreateTemp("", "test-config-*.toml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}
	tmpFile.Close()

	// Reset flags for clean test
	pflag.CommandLine = pflag.NewFlagSet("test", pflag.ContinueOnError)

	config := &TestConfig{
		ConfigFile:    tmpFile.Name(),
		ListenAddress: "127.0.0.1", // default
		ListenPort:    8080,        // default
		Debug:         false,       // default
	}

	config.AddFlags(pflag.CommandLine)

	// Parse with no command line flags (should use config file values)
	if err := pflag.CommandLine.Parse([]string{}); err != nil {
		t.Fatalf("Failed to parse flags: %v", err)
	}

	loader := NewConfigLoader()
	loader.SetConfigFile(config.ConfigFile)
	loader.SetDefaults(map[string]any{
		"listen-address": "127.0.0.1",
		"listen-port":    8080,
		"debug":          false,
	})

	if err := loader.LoadConfig(config); err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify config file values were loaded
	if config.ListenAddress != "192.168.1.100" {
		t.Errorf("Expected ListenAddress to be '192.168.1.100', got '%s'", config.ListenAddress)
	}
	if config.ListenPort != 9090 {
		t.Errorf("Expected ListenPort to be 9090, got %d", config.ListenPort)
	}
	if config.Debug != true {
		t.Errorf("Expected Debug to be true, got %v", config.Debug)
	}
	if config.ConfigFile != tmpFile.Name() {
		t.Errorf("Expected ConfigFile to be preserved, got '%s'", config.ConfigFile)
	}
}

func TestConfigLoader_FlagPrecedence(t *testing.T) {
	// Create a temporary config file
	configContent := `
listen-address = "192.168.1.100"
listen-port = 9090
debug = true
`
	tmpFile, err := os.CreateTemp("", "test-config-*.toml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}
	tmpFile.Close()

	// Reset flags for clean test
	pflag.CommandLine = pflag.NewFlagSet("test", pflag.ContinueOnError)

	config := &TestConfig{
		ConfigFile:    tmpFile.Name(),
		ListenAddress: "127.0.0.1", // default
		ListenPort:    8080,        // default
		Debug:         false,       // default
	}

	config.AddFlags(pflag.CommandLine)

	// Parse with explicit flag (should override config file)
	if err := pflag.CommandLine.Parse([]string{"--listen-port", "7777"}); err != nil {
		t.Fatalf("Failed to parse flags: %v", err)
	}

	loader := NewConfigLoader()
	loader.SetConfigFile(config.ConfigFile)
	loader.SetDefaults(map[string]any{
		"listen-address": "127.0.0.1",
		"listen-port":    8080,
		"debug":          false,
	})

	if err := loader.LoadConfig(config); err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify precedence: explicit flag > config file > defaults
	if config.ListenAddress != "192.168.1.100" {
		t.Errorf("Expected ListenAddress from config file: '192.168.1.100', got '%s'", config.ListenAddress)
	}
	if config.ListenPort != 7777 {
		t.Errorf("Expected ListenPort from explicit flag: 7777, got %d", config.ListenPort)
	}
	if config.Debug != true {
		t.Errorf("Expected Debug from config file: true, got %v", config.Debug)
	}
}

func TestStandardConfigPattern(t *testing.T) {
	// Create a temporary config file
	configContent := `
listen-address = "10.0.0.1"
listen-port = 5555
debug = false
`
	tmpFile, err := os.CreateTemp("", "test-config-*.toml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}
	tmpFile.Close()

	// Reset flags for clean test
	pflag.CommandLine = pflag.NewFlagSet("test", pflag.ContinueOnError)

	config := &TestConfig{
		ListenAddress: "127.0.0.1", // default
		ListenPort:    8080,        // default
		Debug:         true,        // default
	}

	config.AddFlags(pflag.CommandLine)

	// Parse with no command line flags
	if err := pflag.CommandLine.Parse([]string{}); err != nil {
		t.Fatalf("Failed to parse flags: %v", err)
	}

	defaults := map[string]any{
		"listen-address": "127.0.0.1",
		"listen-port":    8080,
		"debug":          true,
	}

	// Use the convenience function
	if err := StandardConfigPattern(config, tmpFile.Name(), defaults); err != nil {
		t.Fatalf("Failed to load config using StandardConfigPattern: %v", err)
	}

	// Verify config file values override defaults
	if config.ListenAddress != "10.0.0.1" {
		t.Errorf("Expected ListenAddress to be '10.0.0.1', got '%s'", config.ListenAddress)
	}
	if config.ListenPort != 5555 {
		t.Errorf("Expected ListenPort to be 5555, got %d", config.ListenPort)
	}
	if config.Debug != false {
		t.Errorf("Expected Debug to be false, got %v", config.Debug)
	}
}

func TestConfigLoader_FlagNameMapping(t *testing.T) {
	// This test specifically validates the dummy.switch-count issue
	type DummyTestConfig struct {
		DummySwitchCount uint `mapstructure:"switch-count"`
	}

	type TestConfig struct {
		Dummy DummyTestConfig `mapstructure:"dummy"`
	}
	addFlags := func(fs *pflag.FlagSet, config *TestConfig) {
		fs.UintVar(&config.Dummy.DummySwitchCount, "dummy.switch-count", config.Dummy.DummySwitchCount, "Number of dummy switches")
	}

	// Reset flags for clean test
	pflag.CommandLine = pflag.NewFlagSet("test", pflag.ContinueOnError)

	config := &TestConfig{
		Dummy: DummyTestConfig{
			DummySwitchCount: 4, // default
		},
	}

	addFlags(pflag.CommandLine, config)

	// Parse with explicit flag (the problematic case)
	if err := pflag.CommandLine.Parse([]string{"--dummy.switch-count", "8"}); err != nil {
		t.Fatalf("Failed to parse flags: %v", err)
	}

	loader := NewConfigLoader()
	loader.SetDefaults(map[string]any{
		"dummy.switch-count": 4,
	})

	if err := loader.LoadConfig(config); err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify the flag with hyphen was correctly mapped to underscore key
	if config.Dummy.DummySwitchCount != 8 {
		t.Errorf("Expected DummySwitchCount to be 8 from explicit flag, got %d", config.Dummy.DummySwitchCount)
	}
}
