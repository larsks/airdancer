package api

import (
	_ "embed"
	"os"
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

	if len(config.Collections) != 0 {
		t.Errorf("NewConfig() Collections = %v, want empty map", config.Collections)
	}

	if len(config.Switches) != 0 {
		t.Errorf("NewConfig() Switches = %v, want empty map", config.Switches)
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
	}

	for _, flagName := range expectedFlags {
		if flag := fs.Lookup(flagName); flag == nil {
			t.Errorf("AddFlags() did not add flag %s", flagName)
		}
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

	// Verify that the embedded test TOML config content is non-empty
	if len(testConfigTOML) == 0 {
		t.Error("Embedded test TOML config should not be empty")
	}

	// Create a temporary file with the embedded test config
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

	// Load and parse the config to verify specific values
	config := NewConfig()

	// Save original command line flags
	originalFlags := pflag.CommandLine
	defer func() { pflag.CommandLine = originalFlags }()

	// Create a clean flag set for testing and add flags (but don't set them explicitly)
	pflag.CommandLine = pflag.NewFlagSet("test", pflag.ContinueOnError)
	config.AddFlags(pflag.CommandLine)

	// Set ConfigFile AFTER AddFlags to prevent it from being overwritten by flag defaults
	config.ConfigFile = tmpFile.Name()

	// Load the config from the embedded test file
	err = config.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load embedded test config: %v", err)
	}

	// Verify specific configuration values from the test-config.toml file
	if config.ListenAddress != "127.0.0.1" {
		t.Errorf("Expected ListenAddress to be '127.0.0.1', got %q", config.ListenAddress)
	}

	if config.ListenPort != 9090 {
		t.Errorf("Expected ListenPort to be 9090, got %d", config.ListenPort)
	}

	// Check collections
	expectedCollections := 3
	if len(config.Collections) != expectedCollections {
		t.Errorf("Expected %d collections, got %d", expectedCollections, len(config.Collections))
	}

	// Check switches
	expectedSwitches := 4
	if len(config.Switches) != expectedSwitches {
		t.Errorf("Expected %d switches, got %d", expectedSwitches, len(config.Switches))
	}

	// Verify specific collection exists
	dummyCollection, exists := config.Collections["dummy-collection"]
	if !exists {
		t.Error("Expected to find dummy-collection in collections")
	} else if dummyCollection.Driver != "dummy" {
		t.Errorf("Expected dummy-collection driver to be 'dummy', got %s", dummyCollection.Driver)
	}

	// Verify specific switch exists
	switch1, exists := config.Switches["switch1"]
	if !exists {
		t.Error("Expected to find switch1 in switches")
	} else if switch1.Spec != "dummy-collection.0" {
		t.Errorf("Expected switch1 spec to be 'dummy-collection.0', got %s", switch1.Spec)
	}

	// Verify that the embedded invalid TOML config content is non-empty
	if len(invalidConfigTOML) == 0 {
		t.Error("Embedded invalid TOML config should not be empty")
	}

	// Test that we can access the embedded TOML content without filesystem operations
	t.Logf("Embedded test TOML config size: %d bytes", len(testConfigTOML))
	t.Logf("Embedded invalid TOML config size: %d bytes", len(invalidConfigTOML))
}
