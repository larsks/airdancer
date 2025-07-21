package cli

import (
	"os"
	"testing"

	"github.com/spf13/pflag"
)

// MockConfig implements Configurable for testing
type MockConfig struct {
	ConfigFile string
	TestValue  string
}

func (m *MockConfig) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&m.ConfigFile, "config", "", "Config file")
	fs.StringVar(&m.TestValue, "test-value", "default", "Test value")
}

func (m *MockConfig) LoadConfigWithFlagSet(fs *pflag.FlagSet) error {
	// Simple mock - just return success
	return nil
}

// MockHandler implements CommandHandler for testing
type MockHandler struct {
	StartCalled bool
	StartError  error
}

func (m *MockHandler) Start(config Configurable) error {
	m.StartCalled = true
	return m.StartError
}

func TestParseArgsStandard_Version(t *testing.T) {
	cli := NewBaseCLI(os.Stdout, os.Stderr)
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)

	args := []string{"--version"}
	cmdArgs, err := cli.ParseArgsStandardWithFlagSet(args, func() Configurable { return &MockConfig{} }, fs)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if cmdArgs.Command != "version" {
		t.Errorf("Expected command 'version', got '%s'", cmdArgs.Command)
	}
}

func TestParseArgsStandard_Start(t *testing.T) {
	cli := NewBaseCLI(os.Stdout, os.Stderr)
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)

	args := []string{"--test-value", "custom"}
	cmdArgs, err := cli.ParseArgsStandardWithFlagSet(args, func() Configurable { return &MockConfig{} }, fs)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if cmdArgs.Command != "start" {
		t.Errorf("Expected command 'start', got '%s'", cmdArgs.Command)
	}

	if config, ok := cmdArgs.Config.(*MockConfig); ok {
		if config.TestValue != "custom" {
			t.Errorf("Expected TestValue 'custom', got '%s'", config.TestValue)
		}
	} else {
		t.Error("Config is not of expected type")
	}
}

func TestExecute_Version(t *testing.T) {
	cli := NewBaseCLI(os.Stdout, os.Stderr)
	handler := &MockHandler{}

	cmdArgs := &CommandArgs{
		Command: "version",
		Config:  &MockConfig{},
	}

	err := cli.Execute(cmdArgs, handler)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if handler.StartCalled {
		t.Error("Start should not have been called for version command")
	}
}

func TestExecute_Start(t *testing.T) {
	cli := NewBaseCLI(os.Stdout, os.Stderr)
	handler := &MockHandler{}

	cmdArgs := &CommandArgs{
		Command: "start",
		Config:  &MockConfig{},
	}

	err := cli.Execute(cmdArgs, handler)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !handler.StartCalled {
		t.Error("Start should have been called for start command")
	}
}

func TestExecute_UnknownCommand(t *testing.T) {
	cli := NewBaseCLI(os.Stdout, os.Stderr)
	handler := &MockHandler{}

	cmdArgs := &CommandArgs{
		Command: "unknown",
		Config:  &MockConfig{},
	}

	err := cli.Execute(cmdArgs, handler)
	if err == nil {
		t.Fatal("Expected error for unknown command")
	}

	if handler.StartCalled {
		t.Error("Start should not have been called for unknown command")
	}
}
