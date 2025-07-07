package main

import (
	"log"
	"os"

	_ "github.com/larsks/airdancer/internal/logsetup"
	"github.com/larsks/airdancer/internal/ui"
	"github.com/larsks/airdancer/internal/version"
	"github.com/spf13/pflag"
)

func main() {
	versionFlag := pflag.Bool("version", false, "Show version and exit")

	cfg := ui.NewConfig()
	cfg.AddFlags(pflag.CommandLine)
	pflag.Parse()

	if *versionFlag {
		version.ShowVersion()
		os.Exit(0)
	}

	if err := cfg.LoadConfig(); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	srv := ui.NewUIServer(cfg)

	if err := srv.Start(); err != nil {
		log.Fatalf("UI server failed: %v", err)
	}
}
