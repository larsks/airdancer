package config

import (
	_ "embed"
	"os"
	"testing"

	"github.com/spf13/pflag"
)

//go:embed testdata/test-config.toml
var testConfigTOML string

//go:embed testdata/flag-precedence-config.toml
var flagPrecedenceConfigTOML string

//go:embed testdata/standard-config.toml
var standardConfigTOML string

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
	tmpFile, err := os.CreateTemp("", "test-config-*.toml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(testConfigTOML); err != nil {
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
	tmpFile, err := os.CreateTemp("", "test-config-*.toml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(flagPrecedenceConfigTOML); err != nil {
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
	tmpFile, err := os.CreateTemp("", "test-config-*.toml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(standardConfigTOML); err != nil {
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

func TestConfigLoader_EnvironmentVariables(t *testing.T) {
	// Set test environment variables
	os.Setenv("TEST_USERNAME", "testuser")
	os.Setenv("TEST_PASSWORD", "secret123")
	os.Setenv("TEST_PORT", "9999")
	defer func() {
		os.Unsetenv("TEST_USERNAME")
		os.Unsetenv("TEST_PASSWORD")
		os.Unsetenv("TEST_PORT")
	}()

	// Create config with environment variables
	configContent := `
listen-address = "$TEST_USERNAME"
listen-port = 8080
debug = true

[imap]
username = "${TEST_USERNAME}"
password = "${TEST_PASSWORD}"
server = "mail.example.com"
port = 993
`

	// Create a temporary config file
	tmpFile, err := os.CreateTemp("", "test-env-config-*.toml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}
	tmpFile.Close()

	// Define test config struct with IMAP section
	type IMAPConfig struct {
		Username string `mapstructure:"username"`
		Password string `mapstructure:"password"`
		Server   string `mapstructure:"server"`
		Port     int    `mapstructure:"port"`
	}

	type EnvTestConfig struct {
		ConfigFile    string     `mapstructure:"config-file"`
		ListenAddress string     `mapstructure:"listen-address"`
		ListenPort    int        `mapstructure:"listen-port"`
		Debug         bool       `mapstructure:"debug"`
		IMAP          IMAPConfig `mapstructure:"imap"`
	}

	// Reset flags for clean test
	pflag.CommandLine = pflag.NewFlagSet("test", pflag.ContinueOnError)

	config := &EnvTestConfig{
		ConfigFile:    tmpFile.Name(),
		ListenAddress: "127.0.0.1",
		ListenPort:    8080,
		Debug:         false,
	}

	// Parse with no command line flags
	if err := pflag.CommandLine.Parse([]string{}); err != nil {
		t.Fatalf("Failed to parse flags: %v", err)
	}

	loader := NewConfigLoader()
	loader.SetConfigFile(config.ConfigFile)

	if err := loader.LoadConfig(config); err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify environment variables were expanded
	if config.ListenAddress != "testuser" {
		t.Errorf("Expected ListenAddress to be 'testuser' (expanded from $TEST_USERNAME), got '%s'", config.ListenAddress)
	}
	if config.IMAP.Username != "testuser" {
		t.Errorf("Expected IMAP.Username to be 'testuser' (expanded from ${TEST_USERNAME}), got '%s'", config.IMAP.Username)
	}
	if config.IMAP.Password != "secret123" {
		t.Errorf("Expected IMAP.Password to be 'secret123' (expanded from ${TEST_PASSWORD}), got '%s'", config.IMAP.Password)
	}
	if config.IMAP.Server != "mail.example.com" {
		t.Errorf("Expected IMAP.Server to remain 'mail.example.com' (no env var), got '%s'", config.IMAP.Server)
	}
	if config.IMAP.Port != 993 {
		t.Errorf("Expected IMAP.Port to remain 993 (no env var), got %d", config.IMAP.Port)
	}
}

func TestConfigLoader_EnvironmentVariables_NotSet(t *testing.T) {
	// Ensure test env vars are not set
	os.Unsetenv("NONEXISTENT_VAR")

	// Create config with non-existent environment variable
	configContent := `
listen-address = "${NONEXISTENT_VAR}"
listen-port = 8080
`

	// Create a temporary config file
	tmpFile, err := os.CreateTemp("", "test-noenv-config-*.toml")
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
		ListenAddress: "127.0.0.1",
		ListenPort:    8080,
		Debug:         false,
	}

	// Parse with no command line flags
	if err := pflag.CommandLine.Parse([]string{}); err != nil {
		t.Fatalf("Failed to parse flags: %v", err)
	}

	loader := NewConfigLoader()
	loader.SetConfigFile(config.ConfigFile)

	if err := loader.LoadConfig(config); err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify unset environment variables remain as original text
	if config.ListenAddress != "${NONEXISTENT_VAR}" {
		t.Errorf("Expected ListenAddress to remain '${NONEXISTENT_VAR}' (unset env var), got '%s'", config.ListenAddress)
	}
}
