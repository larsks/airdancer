package main

import (
	"log"
	"os"

	flag "github.com/spf13/pflag"

	"github.com/larsks/airdancer/internal/monitor"
	"github.com/larsks/airdancer/internal/version"
)

func main() {
	versionFlag := flag.Bool("version", false, "Show version and exit")

	// Create config with defaults
	config := monitor.NewConfig()

	// Add command-line flags
	configFile := flag.String("config", "", "Path to the configuration file (supports JSON, YAML, TOML)")
	config.AddFlags(flag.CommandLine)

	flag.Parse()

	if *versionFlag {
		version.ShowVersion()
		os.Exit(0)
	}

	// Load configuration using viper
	err := config.LoadConfig(*configFile)
	if err != nil {
		// Only fail if config file was explicitly specified but couldn't be loaded
		if *configFile != "" {
			log.Fatalf("failed to load config: %v", err)
		}
		log.Printf("using default configuration: %v", err)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		log.Fatalf("configuration error: %v", err)
	}

	// Create and start monitor
	emailMonitor, err := monitor.NewEmailMonitor(*config)
	if err != nil {
		log.Fatalf("failed to create monitor: %v", err)
	}

	// Start monitoring (this blocks)
	emailMonitor.Start()
}