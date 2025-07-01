package main

import (
	"log"

	"github.com/larsks/airdancer/internal/ui"
	"github.com/spf13/pflag"
)

func main() {
	cfg := ui.NewConfig()
	cfg.AddFlags(pflag.CommandLine)
	pflag.Parse()

	if err := cfg.LoadConfig(); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	srv := ui.NewUIServer(cfg)

	if err := srv.Start(); err != nil {
		log.Fatalf("UI server failed: %v", err)
	}
}