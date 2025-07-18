package main

import (
	"log"
	"os"

	"github.com/larsks/airdancer/internal/buttonwatcher"
	"github.com/larsks/airdancer/internal/version"
	"github.com/spf13/pflag"
)

func main() {
	versionFlag := pflag.Bool("version", false, "Show version and exit")

	cfg := buttonwatcher.NewConfig()
	cfg.AddFlags(pflag.CommandLine)
	pflag.Parse()

	if *versionFlag {
		version.ShowVersion()
		os.Exit(0)
	}

	if err := cfg.LoadConfig(); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if err := cfg.Validate(); err != nil {
		log.Fatalf("Config validation failed: %v", err)
	}

	monitor := buttonwatcher.NewButtonMonitor()
	defer monitor.Close() //nolint:errcheck

	// Set global configuration for defaults
	monitor.SetGlobalConfig(cfg)

	for _, buttonCfg := range cfg.Buttons {
		if err := monitor.AddButtonFromConfig(buttonCfg); err != nil {
			log.Fatalf("failed to add button %s to monitor: %v", buttonCfg.Name, err)
		}
	}

	if err := monitor.Start(); err != nil {
		log.Fatalf("failed to start monitor: %v", err)
	}
}
