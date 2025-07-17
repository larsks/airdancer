package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/larsks/airdancer/internal/api"
	"github.com/spf13/pflag"
)

func TestParseArgs(t *testing.T) {
	// Save original working directory
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(oldWd)

	// Create temporary directory for testing
	tempDir := t.TempDir()
	os.Chdir(tempDir)

	tests := []struct {
		name        string
		args        []string
		want        *CommandArgs
		wantErr     bool
		wantCommand string
	}{
		{
			name:        "no arguments starts server",
			args:        []string{},
			wantCommand: "start",
			wantErr:     false,
		},
		{
			name:        "version flag",
			args:        []string{"--version"},
			wantCommand: "version",
			wantErr:     false,
		},
		{
			name:        "listen-address flag",
			args:        []string{"--listen-address", "127.0.0.1"},
			wantCommand: "start",
			wantErr:     false,
		},
		{
			name:        "listen-port flag",
			args:        []string{"--listen-port", "9090"},
			wantCommand: "start",
			wantErr:     false,
		},
		{
			name:        "config file flag with non-existent file",
			args:        []string{"--config", "/tmp/test.toml"},
			wantCommand: "start",
			wantErr:     true,
		},
		{
			name:    "invalid flag",
			args:    []string{"--invalid-flag"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new flag set for each test to avoid conflicts
			fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
			fs.Usage = func() {} // Suppress usage output

			got, err := ParseArgsWithFlagSet(tt.args, fs)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseArgs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Command != tt.wantCommand {
					t.Errorf("ParseArgs() command = %v, want %v", got.Command, tt.wantCommand)
				}
				if got.Config == nil {
					t.Errorf("ParseArgs() config is nil")
				}
			}
		})
	}
}

func TestParseArgsWithConfig(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test-config.toml")

	// Create test config file
	configContent := `listen-address = "192.168.1.100"
listen-port = 9090`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	args := []string{"--config", configFile}
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.Usage = func() {} // Suppress usage output

	got, err := ParseArgsWithFlagSet(args, fs)
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}

	if got.Config.ListenAddress != "192.168.1.100" {
		t.Errorf("ParseArgs() config.ListenAddress = %v, want %v", got.Config.ListenAddress, "192.168.1.100")
	}
	if got.Config.ListenPort != 9090 {
		t.Errorf("ParseArgs() config.ListenPort = %v, want %v", got.Config.ListenPort, 9090)
	}
}

func TestParseArgsWithNonExistentConfig(t *testing.T) {
	args := []string{"--config", "/nonexistent/config.toml"}
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.Usage = func() {} // Suppress usage output

	_, err := ParseArgsWithFlagSet(args, fs)
	if err == nil {
		t.Error("ParseArgs() expected error for non-existent config file")
	}
	if !strings.Contains(err.Error(), "failed to read config file") {
		t.Errorf("ParseArgs() error = %v, want config file read error", err)
	}
}

func TestCLIExecute(t *testing.T) {
	tests := []struct {
		name    string
		cmdArgs *CommandArgs
		wantErr bool
	}{
		{
			name: "version command",
			cmdArgs: &CommandArgs{
				Command: "version",
				Config:  &api.Config{ListenAddress: "127.0.0.1", ListenPort: 8080},
			},
			wantErr: false,
		},
		{
			name: "invalid command",
			cmdArgs: &CommandArgs{
				Command: "invalid",
				Config:  &api.Config{ListenAddress: "127.0.0.1", ListenPort: 8080},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			cli := NewCLI(tt.cmdArgs.Config, &stdout, &stderr)

			err := cli.Execute(tt.cmdArgs)
			if (err != nil) != tt.wantErr {
				t.Errorf("CLI.Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestCLICommandValidation(t *testing.T) {
	// Test that unknown commands return errors
	var stdout, stderr bytes.Buffer
	cli := NewCLI(&api.Config{ListenAddress: "127.0.0.1", ListenPort: 8080}, &stdout, &stderr)

	err := cli.Execute(&CommandArgs{Command: "unknown", Config: &api.Config{}})
	if err == nil {
		t.Error("CLI.Execute() expected error for unknown command")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("CLI.Execute() error = %v, want to contain 'unknown command'", err.Error())
	}
}

func TestConfig(t *testing.T) {
	// Test default values
	cfg := api.NewConfig()
	if cfg.ListenAddress != "" {
		t.Errorf("NewConfig().ListenAddress = %v, want empty string", cfg.ListenAddress)
	}
	if cfg.ListenPort != 8080 {
		t.Errorf("NewConfig().ListenPort = %v, want 8080", cfg.ListenPort)
	}
}

func TestConfigLoadWithDefaults(t *testing.T) {
	tempDir := t.TempDir()

	// Save and restore original working directory
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(oldWd)
	os.Chdir(tempDir)

	cfg := api.NewConfig()

	// Create a separate flag set for this test
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.Usage = func() {} // Suppress usage output

	// Test with non-existent config file (should not error if not explicitly set)
	err = cfg.LoadConfigWithFlagSet(fs)
	if err != nil {
		t.Errorf("LoadConfigWithFlagSet() with non-existent config should not error: %v", err)
	}

	// Should use default values
	if cfg.ListenAddress != "" {
		t.Errorf("LoadConfigWithFlagSet() ListenAddress = %v, want empty string", cfg.ListenAddress)
	}
	if cfg.ListenPort != 8080 {
		t.Errorf("LoadConfigWithFlagSet() ListenPort = %v, want 8080", cfg.ListenPort)
	}
}

func TestConfigLoadWithExplicitFile(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "explicit-config.toml")

	// Create test config file
	configContent := `listen-address = "192.168.1.50"
listen-port = 9999`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	cfg := api.NewConfig()
	cfg.ConfigFile = configFile

	// Create a separate flag set for this test
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.Usage = func() {} // Suppress usage output

	err = cfg.LoadConfigWithFlagSet(fs)
	if err != nil {
		t.Errorf("LoadConfigWithFlagSet() error = %v", err)
	}

	if cfg.ListenAddress != "192.168.1.50" {
		t.Errorf("LoadConfigWithFlagSet() ListenAddress = %v, want 192.168.1.50", cfg.ListenAddress)
	}
	if cfg.ListenPort != 9999 {
		t.Errorf("LoadConfigWithFlagSet() ListenPort = %v, want 9999", cfg.ListenPort)
	}
}

func TestConfigLoadWithNonExistentExplicitFile(t *testing.T) {
	cfg := api.NewConfig()
	cfg.ConfigFile = "/nonexistent/config.toml"

	// Create a separate flag set for this test
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.Usage = func() {} // Suppress usage output

	err := cfg.LoadConfigWithFlagSet(fs)
	if err == nil {
		t.Error("LoadConfigWithFlagSet() expected error for non-existent explicit config file")
	}
	if !strings.Contains(err.Error(), "failed to read config file") {
		t.Errorf("LoadConfigWithFlagSet() error = %v, want config file read error", err)
	}
}

// Benchmark tests
func BenchmarkParseArgs(b *testing.B) {
	args := []string{"--listen-address", "127.0.0.1", "--listen-port", "8080"}

	for i := 0; i < b.N; i++ {
		// Create a new flag set for each benchmark iteration
		fs := pflag.NewFlagSet("benchmark", pflag.ContinueOnError)
		fs.Usage = func() {} // Suppress usage output

		_, err := ParseArgsWithFlagSet(args, fs)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCLIExecute(b *testing.B) {
	// Use version command for benchmark since it doesn't start a server
	cmdArgs := &CommandArgs{
		Command: "version",
		Config:  &api.Config{ListenAddress: "127.0.0.1", ListenPort: 8080},
	}

	var stdout, stderr bytes.Buffer
	cli := NewCLI(cmdArgs.Config, &stdout, &stderr)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stdout.Reset()
		stderr.Reset()
		err := cli.Execute(cmdArgs)
		if err != nil {
			b.Fatal(err)
		}
	}
}
