package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/larsks/airdancer/internal/api"
	"github.com/larsks/airdancer/internal/buttonwatcher"
	"github.com/larsks/airdancer/internal/config"
	"github.com/larsks/airdancer/internal/monitor"
	"github.com/larsks/airdancer/internal/ui"
	"github.com/larsks/airdancer/internal/version"
	"github.com/spf13/pflag"
)

func main() {
	var (
		versionFlag = pflag.Bool("version", false, "Show version and exit")
		configType  = pflag.String("type", "", "Configuration type: api, ui, monitor, or buttons")
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
	case "buttons":
		err = validateButtonsConfig(*configFile)
	default:
		fmt.Fprintf(os.Stderr, "Error: Unknown configuration type '%s'. Must be 'api', 'ui', 'monitor', or 'buttons'\n", *configType)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Validation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Configuration file %s is valid for %s\n", *configFile, *configType)
}

// validateSwitchSpec validates a switch spec format (collection.index)
func validateSwitchSpec(spec string, collectionNames map[string]bool) error {
	parts := strings.Split(spec, ".")
	if len(parts) != 2 {
		return fmt.Errorf("invalid spec format '%s' (expected format: collection.index)", spec)
	}

	collectionName := parts[0]
	indexStr := parts[1]

	if !collectionNames[collectionName] {
		return fmt.Errorf("references unknown collection '%s'", collectionName)
	}

	if _, err := strconv.ParseUint(indexStr, 10, 32); err != nil {
		return fmt.Errorf("invalid switch index '%s' (must be a non-negative integer)", indexStr)
	}

	return nil
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
	fmt.Fprintf(os.Stderr, "  %s --type buttons --config airdancer-buttons.toml\n", os.Args[0])
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
		"listen-address": "",
		"listen-port":    8080,
		"collections":    make(map[string]api.CollectionConfig),
		"switches":       make(map[string]api.SwitchConfig),
	})

	// Use the config loader directly to get strict validation
	if err := loader.LoadConfig(cfg); err != nil {
		return fmt.Errorf("failed to load API configuration: %v", err)
	}

	// Validate basic configuration
	if cfg.ListenPort <= 0 || cfg.ListenPort > 65535 {
		return fmt.Errorf("listen port must be between 1 and 65535, got %d", cfg.ListenPort)
	}

	// Validate collections
	collectionNames := make(map[string]bool)
	for collectionName, collection := range cfg.Collections {
		if collectionName == "" {
			return fmt.Errorf("collection name cannot be empty")
		}

		collectionNames[collectionName] = true

		if collection.Driver == "" {
			return fmt.Errorf("collection %s: driver is required", collectionName)
		}

		if collection.Driver != "dummy" && collection.Driver != "piface" && collection.Driver != "gpio" {
			return fmt.Errorf("collection %s: driver must be 'dummy', 'piface', or 'gpio', got '%s'", collectionName, collection.Driver)
		}

		// Driver-specific validation
		switch collection.Driver {
		case "dummy":
			if driverConfig := collection.DriverConfig; driverConfig != nil {
				if switchCount, ok := driverConfig["switch_count"]; ok {
					if count, ok := switchCount.(int); ok && count <= 0 {
						return fmt.Errorf("collection %s: dummy driver requires switch_count > 0", collectionName)
					}
				}
			}
		case "piface":
			if driverConfig := collection.DriverConfig; driverConfig != nil {
				if spidev, ok := driverConfig["spidev"]; ok {
					if spidevStr, ok := spidev.(string); ok && spidevStr == "" {
						return fmt.Errorf("collection %s: piface driver requires non-empty spidev", collectionName)
					}
				}
			}
		case "gpio":
			if driverConfig := collection.DriverConfig; driverConfig != nil {
				if pins, ok := driverConfig["pins"]; ok {
					if pinsList, ok := pins.([]interface{}); ok && len(pinsList) == 0 {
						return fmt.Errorf("collection %s: gpio driver requires at least one pin", collectionName)
					}
				}
			}
		}
	}

	// Validate switches
	switchNames := make(map[string]bool)
	for switchName, sw := range cfg.Switches {
		if switchName == "" {
			return fmt.Errorf("switch name cannot be empty")
		}

		switchNames[switchName] = true

		if sw.Spec == "" {
			return fmt.Errorf("switch %s: spec is required", switchName)
		}

		// Validate spec format (collection.index)
		if err := validateSwitchSpec(sw.Spec, collectionNames); err != nil {
			return fmt.Errorf("switch %s: %v", switchName, err)
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

func validateButtonsConfig(configFile string) error {
	// Save the original command line flags
	originalFlags := pflag.CommandLine
	defer func() { pflag.CommandLine = originalFlags }()

	// Create a clean flag set for this validation
	pflag.CommandLine = pflag.NewFlagSet("buttons-validation", pflag.ContinueOnError)

	cfg := buttonwatcher.NewConfig()
	cfg.ConfigFile = configFile

	// Add flags using the same method as the application
	cfg.AddFlags(pflag.CommandLine)

	// Parse with empty arguments (no command line flags set)
	if err := pflag.CommandLine.Parse([]string{}); err != nil {
		return fmt.Errorf("failed to parse flags: %v", err)
	}

	// Load configuration
	if err := cfg.LoadConfig(); err != nil {
		return fmt.Errorf("failed to load buttons configuration: %v", err)
	}

	// Use the built-in validation
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("buttons configuration validation failed: %v", err)
	}

	return nil
}
