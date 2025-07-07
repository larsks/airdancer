package main

import (
	"log"
	"os"

	"github.com/larsks/airdancer/internal/api"
	_ "github.com/larsks/airdancer/internal/logsetup"
	"github.com/larsks/airdancer/internal/version"
	"github.com/spf13/pflag"
)

func main() {
	versionFlag := pflag.Bool("version", false, "Show version and exit")

	cfg := api.NewConfig()
	cfg.AddFlags(pflag.CommandLine)
	pflag.Parse()

	if *versionFlag {
		version.ShowVersion()
		os.Exit(0)
	}

	if err := cfg.LoadConfig(); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	srv, err := api.NewServer(cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}
	defer srv.Close() //nolint:errcheck

	if err := srv.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
