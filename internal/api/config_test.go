package api

import (
	_ "embed"
	"os"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

//go:embed testdata/test-config.toml
var testConfigTOML []byte

//go:embed testdata/invalid-config.toml
var invalidConfigTOML []byte

func TestNewConfig(t *testing.T) {
	config := NewConfig()

	// Test default values
	if config.ListenAddress != "" {
		t.Errorf("NewConfig() ListenAddress = %v, want empty string", config.ListenAddress)
	}

	if config.ListenPort != 8080 {
		t.Errorf("NewConfig() ListenPort = %v, want 8080", config.ListenPort)
	}

	if config.Driver != "piface" {
		t.Errorf("NewConfig() Driver = %v, want piface", config.Driver)
	}

	if config.PiFaceConfig.SPIDev != "/dev/spidev0.0" {
		t.Errorf("NewConfig() PiFaceConfig.SPIDev = %v, want /dev/spidev0.0", config.PiFaceConfig.SPIDev)
	}

	if config.DummyConfig.SwitchCount != 4 {
		t.Errorf("NewConfig() DummyConfig.SwitchCount = %v, want 4", config.DummyConfig.SwitchCount)
	}
}

func TestConfigAddFlags(t *testing.T) {
	config := NewConfig()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)

	config.AddFlags(fs)

	// Test that all expected flags are added
	expectedFlags := []string{
		"config",
		"listen-address",
		"listen-port",
		"driver",
		"piface.spidev",
		"gpio.pins",
		"dummy.switch-count",
	}

	for _, flagName := range expectedFlags {
		if flag := fs.Lookup(flagName); flag == nil {
			t.Errorf("AddFlags() did not add flag %s", flagName)
		}
	}

	// Test that help text includes all drivers
	driverFlag := fs.Lookup("driver")
	if driverFlag == nil {
		t.Fatal("driver flag not found")
	}

	helpText := driverFlag.Usage
	if helpText != "Driver to use (piface, gpio, or dummy)" {
		t.Errorf("AddFlags() driver help = %q, want 'Driver to use (piface, gpio, or dummy)'", helpText)
	}
}

func TestConfigLoadConfig(t *testing.T) {
	// Test loading config without file
	config := NewConfig()

	// Clear any existing flags
	pflag.CommandLine = pflag.NewFlagSet("test", pflag.ContinueOnError)
	config.AddFlags(pflag.CommandLine)

	err := config.LoadConfig()
	if err != nil {
		t.Errorf("LoadConfig() without file failed: %v", err)
	}

	// Test loading config with non-existent file
	config2 := NewConfig()
	config2.ConfigFile = "/nonexistent/config.yaml"

	err = config2.LoadConfig()
	if err == nil {
		t.Error("LoadConfig() with non-existent file should fail")
	}
}

func TestConfigLoadConfigWithEmbeddedFile(t *testing.T) {
	// This test uses embedded TOML configuration files to avoid filesystem dependencies
	// and demonstrates that LoadConfig works with valid configuration files.

	// Create a temporary file with embedded content
	tmpFile, err := os.CreateTemp("", "test-config-*.toml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write the embedded test config to the temp file
	if _, err := tmpFile.Write(testConfigTOML); err != nil {
		t.Fatalf("Failed to write embedded config to temp file: %v", err)
	}
	tmpFile.Close()

	// Save original command line flags
	originalFlags := pflag.CommandLine
	defer func() { pflag.CommandLine = originalFlags }()

	config := NewConfig()
	config.ConfigFile = tmpFile.Name()

	// Create a clean flag set for testing
	pflag.CommandLine = pflag.NewFlagSet("test", pflag.ContinueOnError)
	config.AddFlags(pflag.CommandLine)

	// Test that LoadConfig doesn't return an error with a valid TOML config file
	err = config.LoadConfig()
	if err != nil {
		t.Errorf("LoadConfig() with valid embedded TOML config file failed: %v", err)
	}

	// Verify that the embedded config content is non-empty
	if len(testConfigTOML) == 0 {
		t.Error("Embedded test config should not be empty")
	}
}

func TestEmbeddedConfigContent(t *testing.T) {
	// Test that embedded TOML configuration files are properly loaded and accessible

	// Verify that the embedded test TOML config content is non-empty and contains expected content
	if len(testConfigTOML) == 0 {
		t.Error("Embedded test TOML config should not be empty")
	}

	// Check that the test TOML config contains expected configuration keys
	configStr := string(testConfigTOML)
	expectedKeys := []string{"listen-address", "listen-port", "driver", "dummy", "switch_count"}

	for _, key := range expectedKeys {
		if !strings.Contains(configStr, key) {
			t.Errorf("Embedded test TOML config should contain key %q", key)
		}
	}

	// Verify that the embedded invalid TOML config content is non-empty
	if len(invalidConfigTOML) == 0 {
		t.Error("Embedded invalid TOML config should not be empty")
	}

	// Test that we can access the embedded TOML content without filesystem operations
	t.Logf("Embedded test TOML config size: %d bytes", len(testConfigTOML))
	t.Logf("Embedded invalid TOML config size: %d bytes", len(invalidConfigTOML))
}
