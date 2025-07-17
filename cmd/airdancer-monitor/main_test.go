package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/larsks/airdancer/internal/monitor"
	flag "github.com/spf13/pflag"
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
		wantErr     bool
		wantCommand string
	}{
		{
			name:        "version flag",
			args:        []string{"--version"},
			wantCommand: "version",
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
		{
			name:        "no arguments with minimal valid config",
			args:        []string{},
			wantCommand: "start",
			wantErr:     true, // Will fail validation due to missing required config
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new flag set for each test to avoid conflicts
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
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

func TestParseArgsWithValidConfig(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test-config.toml")

	// Create test config file with minimal valid configuration
	configContent := `check-interval-seconds = 60

[imap]
server = "imap.example.com"
port = 993
username = "testuser"
password = "testpass"
use-ssl = true
retry-interval-seconds = 30

[[monitor]]
mailbox = "INBOX"

[[monitor.triggers]]
regex-pattern = "test pattern"
command = "echo 'test command'"
`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	args := []string{"--config", configFile}
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Usage = func() {} // Suppress usage output

	got, err := ParseArgsWithFlagSet(args, fs)
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}

	if got.Config.IMAP.Server != "imap.example.com" {
		t.Errorf("ParseArgs() config.IMAP.Server = %v, want %v", got.Config.IMAP.Server, "imap.example.com")
	}
	if got.Config.IMAP.Port != 993 {
		t.Errorf("ParseArgs() config.IMAP.Port = %v, want %v", got.Config.IMAP.Port, 993)
	}
	if got.Config.CheckInterval == nil || *got.Config.CheckInterval != 60 {
		t.Errorf("ParseArgs() config.CheckInterval = %v, want 60", got.Config.CheckInterval)
	}
}

func TestParseArgsWithNonExistentConfig(t *testing.T) {
	args := []string{"--config", "/nonexistent/config.toml"}
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Usage = func() {} // Suppress usage output

	_, err := ParseArgsWithFlagSet(args, fs)
	if err == nil {
		t.Error("ParseArgs() expected error for non-existent config file")
	}
	if !strings.Contains(err.Error(), "failed to load config") {
		t.Errorf("ParseArgs() error = %v, want config load error", err)
	}
}

func TestParseArgsWithInvalidConfig(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "invalid-config.toml")

	// Create test config file with missing required fields
	configContent := `check-interval-seconds = 60

[imap]
server = ""
port = 993
username = "testuser"
password = "testpass"
`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	args := []string{"--config", configFile}
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Usage = func() {} // Suppress usage output

	_, err = ParseArgsWithFlagSet(args, fs)
	if err == nil {
		t.Error("ParseArgs() expected error for invalid config")
	}
	if !strings.Contains(err.Error(), "configuration error") {
		t.Errorf("ParseArgs() error = %v, want configuration error", err)
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
				Config:  &monitor.Config{},
			},
			wantErr: false,
		},
		{
			name: "invalid command",
			cmdArgs: &CommandArgs{
				Command: "invalid",
				Config:  &monitor.Config{},
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
	cli := NewCLI(&monitor.Config{}, &stdout, &stderr)

	err := cli.Execute(&CommandArgs{Command: "unknown", Config: &monitor.Config{}})
	if err == nil {
		t.Error("CLI.Execute() expected error for unknown command")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("CLI.Execute() error = %v, want to contain 'unknown command'", err.Error())
	}
}

func TestConfig(t *testing.T) {
	// Test default values
	cfg := monitor.NewConfig()
	if cfg.CheckInterval == nil || *cfg.CheckInterval != 30 {
		t.Errorf("NewConfig().CheckInterval = %v, want 30", cfg.CheckInterval)
	}
	if cfg.IMAP.Port != 993 {
		t.Errorf("NewConfig().IMAP.Port = %v, want 993", cfg.IMAP.Port)
	}
	if cfg.IMAP.UseSSL != true {
		t.Errorf("NewConfig().IMAP.UseSSL = %v, want true", cfg.IMAP.UseSSL)
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

	cfg := monitor.NewConfig()

	// Create a separate flag set for this test
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Usage = func() {} // Suppress usage output

	// Test loading config without file (this should work for the loading part)
	err = cfg.LoadConfigWithFlagSet(fs)
	if err != nil {
		t.Errorf("LoadConfigWithFlagSet() should not error during loading: %v", err)
	}

	// Should use default values for what was loaded
	if cfg.CheckInterval == nil || *cfg.CheckInterval != 30 {
		t.Errorf("LoadConfigWithFlagSet() CheckInterval = %v, want 30", cfg.CheckInterval)
	}
	if cfg.IMAP.Port != 993 {
		t.Errorf("LoadConfigWithFlagSet() IMAP.Port = %v, want 993", cfg.IMAP.Port)
	}

	// But validation should fail due to missing required config
	err = cfg.Validate()
	if err == nil {
		t.Error("Config.Validate() expected error due to missing required config")
	}
}

func TestConfigLoadWithExplicitFile(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "explicit-config.toml")

	// Create test config file with minimal valid configuration
	configContent := `check-interval-seconds = 120

[imap]
server = "mail.example.com"
port = 143
username = "testuser"
password = "testpass"
use-ssl = false
retry-interval-seconds = 60

[[monitor]]
mailbox = "INBOX"

[[monitor.triggers]]
regex-pattern = "urgent"
command = "echo 'urgent mail'"
`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	cfg := monitor.NewConfig()
	cfg.ConfigFile = configFile

	// Create a separate flag set for this test
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Usage = func() {} // Suppress usage output

	err = cfg.LoadConfigWithFlagSet(fs)
	if err != nil {
		t.Errorf("LoadConfigWithFlagSet() error = %v", err)
	}

	if cfg.CheckInterval == nil || *cfg.CheckInterval != 120 {
		t.Errorf("LoadConfigWithFlagSet() CheckInterval = %v, want 120", cfg.CheckInterval)
	}
	if cfg.IMAP.Server != "mail.example.com" {
		t.Errorf("LoadConfigWithFlagSet() IMAP.Server = %v, want mail.example.com", cfg.IMAP.Server)
	}
	if cfg.IMAP.Port != 143 {
		t.Errorf("LoadConfigWithFlagSet() IMAP.Port = %v, want 143", cfg.IMAP.Port)
	}
	if cfg.IMAP.UseSSL != false {
		t.Errorf("LoadConfigWithFlagSet() IMAP.UseSSL = %v, want false", cfg.IMAP.UseSSL)
	}
}

func TestConfigLoadWithNonExistentExplicitFile(t *testing.T) {
	cfg := monitor.NewConfig()
	cfg.ConfigFile = "/nonexistent/config.toml"

	// Create a separate flag set for this test
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Usage = func() {} // Suppress usage output

	err := cfg.LoadConfigWithFlagSet(fs)
	if err == nil {
		t.Error("LoadConfigWithFlagSet() expected error for non-existent explicit config file")
	}
	if !strings.Contains(err.Error(), "failed to read config file") {
		t.Errorf("LoadConfigWithFlagSet() error = %v, want config file read error", err)
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func() *monitor.Config
		wantErr   bool
		errMsg    string
	}{
		{
			name: "valid config",
			setupFunc: func() *monitor.Config {
				cfg := monitor.NewConfig()
				cfg.IMAP.Server = "imap.example.com"
				cfg.IMAP.Port = 993
				cfg.IMAP.Username = "testuser"
				cfg.IMAP.Password = "testpass"
				cfg.Monitor = []monitor.MailboxConfig{
					{
						Mailbox: "INBOX",
						Triggers: []monitor.TriggerConfig{
							{
								RegexPattern: "test",
								Command:      "echo test",
							},
						},
					},
				}
				return cfg
			},
			wantErr: false,
		},
		{
			name: "missing server",
			setupFunc: func() *monitor.Config {
				cfg := monitor.NewConfig()
				cfg.IMAP.Server = ""
				cfg.IMAP.Port = 993
				return cfg
			},
			wantErr: true,
			errMsg:  "server is empty",
		},
		{
			name: "invalid port",
			setupFunc: func() *monitor.Config {
				cfg := monitor.NewConfig()
				cfg.IMAP.Server = "imap.example.com"
				cfg.IMAP.Port = -1
				return cfg
			},
			wantErr: true,
			errMsg:  "port is -1",
		},
		{
			name: "no monitor configs",
			setupFunc: func() *monitor.Config {
				cfg := monitor.NewConfig()
				cfg.IMAP.Server = "imap.example.com"
				cfg.IMAP.Port = 993
				cfg.Monitor = []monitor.MailboxConfig{}
				return cfg
			},
			wantErr: true,
			errMsg:  "no monitor configurations",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.setupFunc()
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Config.Validate() error = %v, want to contain %v", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

// Benchmark tests
func BenchmarkParseArgs(b *testing.B) {
	args := []string{"--config", "/tmp/nonexistent.toml"}

	for i := 0; i < b.N; i++ {
		// Create a new flag set for each benchmark iteration
		fs := flag.NewFlagSet("benchmark", flag.ContinueOnError)
		fs.Usage = func() {} // Suppress usage output

		// This will error due to missing config, but we're testing parsing speed
		_, _ = ParseArgsWithFlagSet(args, fs)
	}
}

func BenchmarkCLIExecute(b *testing.B) {
	// Use version command for benchmark since it doesn't start a monitor
	cmdArgs := &CommandArgs{
		Command: "version",
		Config:  &monitor.Config{},
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
