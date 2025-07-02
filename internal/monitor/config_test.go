package monitor

import (
	_ "embed"
	"errors"
	"os"
	"testing"

	"github.com/spf13/pflag"
)

//go:embed testdata/test-config.toml
var testConfigContent string

func TestNewConfig(t *testing.T) {
	config := NewConfig()

	// Test default values
	if config.IMAP.Port != 993 {
		t.Errorf("Expected IMAP port to be 993, got %d", config.IMAP.Port)
	}

	if !config.IMAP.UseSSL {
		t.Error("Expected UseSSL to be true by default")
	}

	if config.IMAP.Mailbox != "INBOX" {
		t.Errorf("Expected default mailbox to be 'INBOX', got %q", config.IMAP.Mailbox)
	}

	if config.Monitor.CheckInterval != 30 {
		t.Errorf("Expected default check interval to be 30, got %d", config.Monitor.CheckInterval)
	}

	// Test that optional fields are empty by default
	if config.IMAP.Server != "" {
		t.Errorf("Expected IMAP server to be empty by default, got %q", config.IMAP.Server)
	}

	if config.IMAP.Username != "" {
		t.Errorf("Expected IMAP username to be empty by default, got %q", config.IMAP.Username)
	}

	if config.Monitor.RegexPattern != "" {
		t.Errorf("Expected regex pattern to be empty by default, got %q", config.Monitor.RegexPattern)
	}
}

func TestConfigAddFlags(t *testing.T) {
	config := NewConfig()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)

	config.AddFlags(fs)

	// Test that all expected flags are added
	expectedFlags := []string{
		"config",
		"imap.server",
		"imap.port",
		"imap.username",
		"imap.password",
		"imap.use-ssl",
		"imap.mailbox",
		"monitor.regex-pattern",
		"monitor.command",
		"monitor.check-interval",
	}

	for _, flagName := range expectedFlags {
		if flag := fs.Lookup(flagName); flag == nil {
			t.Errorf("AddFlags() did not add flag %s", flagName)
		}
	}

	// Test flag descriptions
	serverFlag := fs.Lookup("imap.server")
	if serverFlag == nil {
		t.Fatal("imap.server flag not found")
	}
	if serverFlag.Usage != "IMAP server address" {
		t.Errorf("Expected server flag usage to be 'IMAP server address', got %q", serverFlag.Usage)
	}

	regexFlag := fs.Lookup("monitor.regex-pattern")
	if regexFlag == nil {
		t.Fatal("monitor.regex-pattern flag not found")
	}
	if regexFlag.Usage != "Regex pattern to match in email bodies" {
		t.Errorf("Expected regex flag usage description, got %q", regexFlag.Usage)
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name           string
		config         *Config
		expectedError  error
		errorSubstring string
	}{
		{
			name: "valid config",
			config: &Config{
				IMAP: IMAPConfig{
					Server:   "imap.example.com",
					Port:     993,
					Username: "user@example.com",
					Password: "password",
					UseSSL:   true,
					Mailbox:  "INBOX",
				},
				Monitor: MonitorConfig{
					RegexPattern:  "test.*pattern",
					Command:       "echo 'matched'",
					CheckInterval: 30,
				},
			},
			expectedError: nil,
		},
		{
			name: "missing IMAP server",
			config: &Config{
				IMAP: IMAPConfig{
					Server: "", // Missing server
					Port:   993,
				},
				Monitor: MonitorConfig{
					RegexPattern: "test.*pattern",
				},
			},
			expectedError: ErrMissingIMAPServer,
		},
		{
			name: "invalid IMAP port",
			config: &Config{
				IMAP: IMAPConfig{
					Server: "imap.example.com",
					Port:   0, // Invalid port
				},
				Monitor: MonitorConfig{
					RegexPattern: "test.*pattern",
				},
			},
			expectedError: ErrInvalidIMAPPort,
		},
		{
			name: "missing regex pattern",
			config: &Config{
				IMAP: IMAPConfig{
					Server: "imap.example.com",
					Port:   993,
				},
				Monitor: MonitorConfig{
					RegexPattern: "", // Missing pattern
				},
			},
			expectedError: ErrMissingRegexPattern,
		},
		{
			name: "minimal valid config",
			config: &Config{
				IMAP: IMAPConfig{
					Server: "imap.example.com",
					Port:   993,
				},
				Monitor: MonitorConfig{
					RegexPattern: ".*",
				},
			},
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectedError == nil {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("Expected error %v, got nil", tt.expectedError)
				} else if !errors.Is(err, tt.expectedError) {
					t.Errorf("Expected error %v, got %v", tt.expectedError, err)
				}
			}
		})
	}
}

func TestConfigLoadConfigFromStruct(t *testing.T) {
	// Create a temporary config file using embedded content
	tmpFile, err := os.CreateTemp("", "monitor-config-*.toml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(testConfigContent); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
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

	// Test that LoadConfigFromStruct doesn't return an error with a valid config file
	err = config.LoadConfigFromStruct()
	if err != nil {
		t.Errorf("LoadConfigFromStruct() with valid config file failed: %v", err)
	}

	// Verify that values were loaded correctly
	if config.IMAP.Server != "test.example.com" {
		t.Errorf("Expected IMAP server to be 'test.example.com', got %q", config.IMAP.Server)
	}

	if config.IMAP.Port != 143 {
		t.Errorf("Expected IMAP port to be 143, got %d", config.IMAP.Port)
	}

	if config.IMAP.UseSSL != false {
		t.Errorf("Expected UseSSL to be false, got %v", config.IMAP.UseSSL)
	}

	if config.Monitor.RegexPattern != "urgent.*alert" {
		t.Errorf("Expected regex pattern to be 'urgent.*alert', got %q", config.Monitor.RegexPattern)
	}

	if config.Monitor.CheckInterval != 60 {
		t.Errorf("Expected check interval to be 60, got %d", config.Monitor.CheckInterval)
	}
}

func TestConfigLoadConfigFromStructWithInvalidFile(t *testing.T) {
	config := NewConfig()
	config.ConfigFile = "/nonexistent/config.toml"

	// Save original command line flags
	originalFlags := pflag.CommandLine
	defer func() { pflag.CommandLine = originalFlags }()

	// Create a clean flag set for testing
	pflag.CommandLine = pflag.NewFlagSet("test", pflag.ContinueOnError)
	config.AddFlags(pflag.CommandLine)

	err := config.LoadConfigFromStruct()
	if err == nil {
		t.Error("Expected error when loading non-existent config file")
	}
}

func TestConfigLoadConfigFromStructUsesDefaults(t *testing.T) {
	config := NewConfig()
	// Don't set ConfigFile, so it should use defaults

	// Save original command line flags
	originalFlags := pflag.CommandLine
	defer func() { pflag.CommandLine = originalFlags }()

	// Create a clean flag set for testing
	pflag.CommandLine = pflag.NewFlagSet("test", pflag.ContinueOnError)
	config.AddFlags(pflag.CommandLine)

	err := config.LoadConfigFromStruct()
	if err != nil {
		t.Errorf("LoadConfigFromStruct() without config file failed: %v", err)
	}

	// Verify defaults are preserved
	if config.IMAP.Port != 993 {
		t.Errorf("Expected default IMAP port 993, got %d", config.IMAP.Port)
	}

	if !config.IMAP.UseSSL {
		t.Error("Expected default UseSSL to be true")
	}

	if config.IMAP.Mailbox != "INBOX" {
		t.Errorf("Expected default mailbox 'INBOX', got %q", config.IMAP.Mailbox)
	}

	if config.Monitor.CheckInterval != 30 {
		t.Errorf("Expected default check interval 30, got %d", config.Monitor.CheckInterval)
	}
}

// Test edge cases and boundary conditions
func TestConfigValidateEdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		configBuilder func() *Config
		shouldPass    bool
	}{
		{
			name: "negative port",
			configBuilder: func() *Config {
				c := NewConfig()
				c.IMAP.Server = "imap.example.com"
				c.IMAP.Port = -1
				c.Monitor.RegexPattern = "test"
				return c
			},
			shouldPass: false,
		},
		{
			name: "very large port",
			configBuilder: func() *Config {
				c := NewConfig()
				c.IMAP.Server = "imap.example.com"
				c.IMAP.Port = 99999
				c.Monitor.RegexPattern = "test"
				return c
			},
			shouldPass: true, // Port validation only checks for zero
		},
		{
			name: "empty regex pattern",
			configBuilder: func() *Config {
				c := NewConfig()
				c.IMAP.Server = "imap.example.com"
				c.IMAP.Port = 993
				c.Monitor.RegexPattern = ""
				return c
			},
			shouldPass: false,
		},
		{
			name: "whitespace-only regex pattern",
			configBuilder: func() *Config {
				c := NewConfig()
				c.IMAP.Server = "imap.example.com"
				c.IMAP.Port = 993
				c.Monitor.RegexPattern = "   "
				return c
			},
			shouldPass: true, // Current validation doesn't trim whitespace
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := tt.configBuilder()
			err := config.Validate()

			if tt.shouldPass && err != nil {
				t.Errorf("Expected validation to pass, got error: %v", err)
			}
			if !tt.shouldPass && err == nil {
				t.Error("Expected validation to fail, but it passed")
			}
		})
	}
}
