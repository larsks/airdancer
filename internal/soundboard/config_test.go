package soundboard

import (
	"testing"

	"github.com/spf13/pflag"
)

func TestNewConfig(t *testing.T) {
	cfg := NewConfig()

	if cfg.ListenAddress != "" {
		t.Errorf("expected empty listen address, got %q", cfg.ListenAddress)
	}

	if cfg.ListenPort != 8082 {
		t.Errorf("expected listen port 8082, got %d", cfg.ListenPort)
	}

	if cfg.SoundDirectory != "./sounds" {
		t.Errorf("expected sound directory './sounds', got %q", cfg.SoundDirectory)
	}

	if cfg.ItemsPerPage != 20 {
		t.Errorf("expected items per page 20, got %d", cfg.ItemsPerPage)
	}
}

func TestAddFlags(t *testing.T) {
	cfg := NewConfig()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)

	cfg.AddFlags(fs)

	// Test that flags were added
	flags := []string{"config", "listen-address", "listen-port", "sound-directory", "items-per-page"}
	for _, flagName := range flags {
		if fs.Lookup(flagName) == nil {
			t.Errorf("flag %s was not added", flagName)
		}
	}
}

func TestFlagDefaults(t *testing.T) {
	cfg := NewConfig()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)

	cfg.AddFlags(fs)

	// Parse empty args to get defaults
	err := fs.Parse([]string{})
	if err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	// Check that the config struct values match flag defaults
	if cfg.ListenPort != 8082 {
		t.Errorf("expected default listen port 8082, got %d", cfg.ListenPort)
	}

	if cfg.SoundDirectory != "./sounds" {
		t.Errorf("expected default sound directory './sounds', got %q", cfg.SoundDirectory)
	}

	if cfg.ItemsPerPage != 20 {
		t.Errorf("expected default items per page 20, got %d", cfg.ItemsPerPage)
	}
}
