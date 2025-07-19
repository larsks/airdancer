package main

import (
	"bytes"
	"testing"

	"github.com/larsks/airdancer/internal/soundboard"
	"github.com/spf13/pflag"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectCmd   string
		expectError bool
	}{
		{
			name:        "version flag",
			args:        []string{"--version"},
			expectCmd:   "version",
			expectError: false,
		},
		{
			name:        "start command with config",
			args:        []string{"--config", ""},
			expectCmd:   "start",
			expectError: false,
		},
		{
			name:        "start command with port",
			args:        []string{"--listen-port", "8083"},
			expectCmd:   "start",
			expectError: false,
		},
		{
			name:        "start command with sound directory",
			args:        []string{"--sound-directory", "/path/to/sounds"},
			expectCmd:   "start",
			expectError: false,
		},
		{
			name:        "no args (start with defaults)",
			args:        []string{},
			expectCmd:   "start",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new flag set for each test to avoid conflicts
			fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
			fs.SetOutput(bytes.NewBuffer(nil)) // Suppress error output

			cmdArgs, err := ParseArgsWithFlagSet(tt.args, fs)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if cmdArgs.Command != tt.expectCmd {
				t.Errorf("expected command %q, got %q", tt.expectCmd, cmdArgs.Command)
			}

			if cmdArgs.Config == nil {
				t.Error("expected config to be non-nil")
			}
		})
	}
}

func TestNewCLI(t *testing.T) {
	cfg := soundboard.NewConfig()
	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)

	cli := NewCLI(cfg, stdout, stderr)

	if cli.config != cfg {
		t.Error("expected config to be set")
	}

	if cli.stdout != stdout {
		t.Error("expected stdout to be set")
	}

	if cli.stderr != stderr {
		t.Error("expected stderr to be set")
	}
}

func TestExecuteVersion(t *testing.T) {
	cfg := soundboard.NewConfig()
	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)

	cli := NewCLI(cfg, stdout, stderr)

	cmdArgs := &CommandArgs{
		Command: "version",
		Config:  cfg,
	}

	err := cli.Execute(cmdArgs)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExecuteUnknownCommand(t *testing.T) {
	cfg := soundboard.NewConfig()
	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)

	cli := NewCLI(cfg, stdout, stderr)

	cmdArgs := &CommandArgs{
		Command: "unknown",
		Config:  cfg,
	}

	err := cli.Execute(cmdArgs)
	if err == nil {
		t.Error("expected error for unknown command")
	}
}
