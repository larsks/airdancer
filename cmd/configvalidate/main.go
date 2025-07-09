package main

import (
	"fmt"
	"os"
	"regexp"

	"github.com/larsks/airdancer/internal/api"
	"github.com/larsks/airdancer/internal/config"
	"github.com/larsks/airdancer/internal/monitor"
	"github.com/larsks/airdancer/internal/ui"
	"github.com/larsks/airdancer/internal/version"
	"github.com/spf13/pflag"
)

func main() {
	var (
		versionFlag = pflag.Bool("version", false, "Show version and exit")
		configType  = pflag.String("type", "", "Configuration type: api, ui, or monitor")
		configFile  = pflag.String("config", "", "Configuration file to validate")
		helpFlag    = pflag.BoolP("help", "h", false, "Show help")
	)

	pflag.Parse()

	if *versionFlag {
		version.ShowVersion()
		os.Exit(0)
	}

	if *helpFlag {
		usage()
		os.Exit(0)
	}

	if *configFile == "" {
		fmt.Fprintf(os.Stderr, "Error: --config flag is required\n\n")
		usage()
		os.Exit(1)
	}

	if *configType == "" {
		fmt.Fprintf(os.Stderr, "Error: --type flag is required\n\n")
		usage()
		os.Exit(1)
	}

	// Check if config file exists
	if _, err := os.Stat(*configFile); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: Configuration file %s does not exist\n", *configFile)
		os.Exit(1)
	}

	var err error
	switch *configType {
	case "api":
		err = validateAPIConfig(*configFile)
	case "ui":
		err = validateUIConfig(*configFile)
	case "monitor":
		err = validateMonitorConfig(*configFile)
	default:
		fmt.Fprintf(os.Stderr, "Error: Unknown configuration type '%s'. Must be 'api', 'ui', or 'monitor'\n", *configType)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Validation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Configuration file %s is valid for %s\n", *configFile, *configType)
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s --type TYPE --config FILE\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "A tool for validating Airdancer configuration files.\n\n")

	fmt.Fprintf(os.Stderr, "Options:\n")
	pflag.PrintDefaults()

	fmt.Fprintf(os.Stderr, "\nExamples:\n")
	fmt.Fprintf(os.Stderr, "  %s --type api --config airdancer-api.toml\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s --type ui --config airdancer-ui.toml\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s --type monitor --config airdancer-monitor.toml\n", os.Args[0])
}

func validateAPIConfig(configFile string) error {
	// Save the original command line flags
	originalFlags := pflag.CommandLine
	defer func() { pflag.CommandLine = originalFlags }()

	// Create a clean flag set for this validation
	pflag.CommandLine = pflag.NewFlagSet("api-validation", pflag.ContinueOnError)

	cfg := api.NewConfig()
	cfg.ConfigFile = configFile

	// Add flags using the same method as the application
	cfg.AddFlags(pflag.CommandLine)

	// Parse with empty arguments (no command line flags set)
	if err := pflag.CommandLine.Parse([]string{}); err != nil {
		return fmt.Errorf("failed to parse flags: %v", err)
	}

	// Create a config loader with strict mode enabled
	loader := config.NewConfigLoader()
	loader.SetConfigFile(configFile)
	loader.SetStrictMode(true) // Enable strict validation to detect unknown fields

	// Set the same defaults as the API config
	loader.SetDefaults(map[string]any{
		"listen-address":     "",
		"listen-port":        8080,
		"driver":             "dummy",
		"piface.spidev":      "/dev/spidev0.0",
		"gpio.pins":          []string{},
		"dummy.switch_count": 4,
	})

	// Use the config loader directly to get strict validation
	if err := loader.LoadConfig(cfg); err != nil {
		return fmt.Errorf("failed to load API configuration: %v", err)
	}

	// Validate required fields and reasonable values
	if cfg.Driver == "" {
		return fmt.Errorf("driver is required")
	}

	if cfg.Driver != "dummy" && cfg.Driver != "piface" && cfg.Driver != "gpio" {
		return fmt.Errorf("driver must be 'dummy', 'piface', or 'gpio', got '%s'", cfg.Driver)
	}

	if cfg.ListenPort <= 0 || cfg.ListenPort > 65535 {
		return fmt.Errorf("listen port must be between 1 and 65535, got %d", cfg.ListenPort)
	}

	// Driver-specific validation
	switch cfg.Driver {
	case "dummy":
		if cfg.DummyConfig.SwitchCount == 0 {
			return fmt.Errorf("dummy driver requires switch_count > 0")
		}
	case "piface":
		if cfg.PiFaceConfig.SPIDev == "" {
			return fmt.Errorf("piface driver requires spidev to be set")
		}
	case "gpio":
		if len(cfg.GPIOConfig.Pins) == 0 {
			return fmt.Errorf("gpio driver requires at least one pin to be specified")
		}
	}

	return nil
}

func validateUIConfig(configFile string) error {
	// Save the original command line flags
	originalFlags := pflag.CommandLine
	defer func() { pflag.CommandLine = originalFlags }()

	// Create a clean flag set for this validation
	pflag.CommandLine = pflag.NewFlagSet("ui-validation", pflag.ContinueOnError)

	cfg := ui.NewConfig()
	cfg.ConfigFile = configFile

	// Add flags using the same method as the application
	cfg.AddFlags(pflag.CommandLine)

	// Parse with empty arguments (no command line flags set)
	if err := pflag.CommandLine.Parse([]string{}); err != nil {
		return fmt.Errorf("failed to parse flags: %v", err)
	}

	// Create a config loader with strict mode enabled
	loader := config.NewConfigLoader()
	loader.SetConfigFile(configFile)
	loader.SetStrictMode(true) // Enable strict validation to detect unknown fields

	// Set the same defaults as the UI config
	loader.SetDefaults(map[string]any{
		"listen-address": "",
		"listen-port":    8081,
		"api-base-url":   "http://localhost:8080",
	})

	// Use the config loader directly to get strict validation
	if err := loader.LoadConfig(cfg); err != nil {
		return fmt.Errorf("failed to load UI configuration: %v", err)
	}

	// Validate required fields and reasonable values
	if cfg.ListenPort <= 0 || cfg.ListenPort > 65535 {
		return fmt.Errorf("listen port must be between 1 and 65535, got %d", cfg.ListenPort)
	}

	if cfg.APIBaseURL == "" {
		return fmt.Errorf("api_base_url is required")
	}

	return nil
}

func validateMonitorConfig(configFile string) error {
	// Save the original command line flags
	originalFlags := pflag.CommandLine
	defer func() { pflag.CommandLine = originalFlags }()

	// Create a clean flag set for this validation
	pflag.CommandLine = pflag.NewFlagSet("monitor-validation", pflag.ContinueOnError)

	cfg := monitor.NewConfig()
	cfg.ConfigFile = configFile

	// Add flags using the same method as the application
	cfg.AddFlags(pflag.CommandLine)

	// Parse with empty arguments (no command line flags set)
	if err := pflag.CommandLine.Parse([]string{}); err != nil {
		return fmt.Errorf("failed to parse flags: %v", err)
	}

	// Use the monitor config's LoadConfigFromStruct method for proper validation
	if err := cfg.LoadConfigFromStruct(); err != nil {
		return fmt.Errorf("failed to load monitor configuration: %v", err)
	}

	// Use the built-in validation
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("monitor configuration validation failed: %v", err)
	}

	// Additional validation for the new multi-mailbox structure
	if err := validateMonitorStructure(cfg); err != nil {
		return fmt.Errorf("monitor structure validation failed: %v", err)
	}

	return nil
}

// validateMonitorStructure performs additional validation on the monitor configuration structure
func validateMonitorStructure(cfg *monitor.Config) error {
	// Validate IMAP configuration
	if cfg.IMAP.Server == "" {
		return fmt.Errorf("IMAP server is required")
	}

	if cfg.IMAP.Port <= 0 || cfg.IMAP.Port > 65535 {
		return fmt.Errorf("IMAP port must be between 1 and 65535, got %d", cfg.IMAP.Port)
	}

	if cfg.IMAP.Username == "" {
		return fmt.Errorf("IMAP username is required")
	}

	if cfg.IMAP.Password == "" {
		return fmt.Errorf("IMAP password is required")
	}

	// Validate global check interval if set
	if cfg.CheckInterval != nil && *cfg.CheckInterval <= 0 {
		return fmt.Errorf("global check_interval_seconds must be positive, got %d", *cfg.CheckInterval)
	}

	// Validate each mailbox configuration
	for i, mailbox := range cfg.Monitor {
		if mailbox.Mailbox == "" {
			return fmt.Errorf("mailbox name is required for monitor %d", i)
		}

		// Validate mailbox-specific check interval if set
		if mailbox.CheckInterval != nil && *mailbox.CheckInterval <= 0 {
			return fmt.Errorf("check_interval_seconds must be positive for mailbox %s, got %d", mailbox.Mailbox, *mailbox.CheckInterval)
		}

		// Validate that mailbox has at least one trigger
		if len(mailbox.Triggers) == 0 {
			return fmt.Errorf("mailbox %s must have at least one trigger", mailbox.Mailbox)
		}

		// Validate each trigger
		for j, trigger := range mailbox.Triggers {
			if trigger.RegexPattern == "" {
				return fmt.Errorf("regex_pattern is required for trigger %d in mailbox %s", j, mailbox.Mailbox)
			}

			// Test if regex pattern is valid
			if _, err := regexp.Compile(trigger.RegexPattern); err != nil {
				return fmt.Errorf("invalid regex pattern '%s' in trigger %d of mailbox %s: %v", trigger.RegexPattern, j, mailbox.Mailbox, err)
			}
		}
	}

	return nil
}
