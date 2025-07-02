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
	config.AddFlags(flag.CommandLine)
	flag.Parse()

	if *versionFlag {
		version.ShowVersion()
		os.Exit(0)
	}

	// Load configuration using the common pattern
	err := config.LoadConfigFromStruct()
	if err != nil {
		// Only fail if config file was explicitly specified but couldn't be loaded
		if config.ConfigFile != "" {
			log.Fatalf("failed to load config: %v", err)
		}
		log.Printf("using default configuration: %v", err)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		log.Fatalf("configuration error: %v", err)
	}

	// Create and start monitor
	emailMonitor, err := monitor.NewEmailMonitorWithDefaults(*config)
	if err != nil {
		log.Fatalf("failed to create monitor: %v", err)
	}

	// Start monitoring (this blocks)
	emailMonitor.Start()
}
