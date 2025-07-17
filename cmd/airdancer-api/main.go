package main

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/larsks/airdancer/internal/api"
	_ "github.com/larsks/airdancer/internal/logsetup"
	"github.com/larsks/airdancer/internal/version"
	"github.com/spf13/pflag"
)

// ServerInterface abstracts the server for testing
type ServerInterface interface {
	Start() error
	Close() error
}

// CLI represents the command line interface for airdancer-api
type CLI struct {
	config *api.Config
	stdout io.Writer
	stderr io.Writer
}

// NewCLI creates a new CLI instance
func NewCLI(cfg *api.Config, stdout, stderr io.Writer) *CLI {
	return &CLI{
		config: cfg,
		stdout: stdout,
		stderr: stderr,
	}
}

// CommandArgs represents parsed command line arguments
type CommandArgs struct {
	Command string
	Config  *api.Config
}

// ParseArgs parses command line arguments using pflag.CommandLine
func ParseArgs(args []string) (*CommandArgs, error) {
	return ParseArgsWithFlagSet(args, pflag.CommandLine)
}

// ParseArgsWithFlagSet parses command line arguments with a custom flag set (for testing)
func ParseArgsWithFlagSet(args []string, fs *pflag.FlagSet) (*CommandArgs, error) {
	// Define flags
	versionFlag := fs.Bool("version", false, "Show version and exit")

	// Config flags
	cfg := api.NewConfig()
	cfg.AddFlags(fs)

	// Parse arguments
	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("failed to parse flags: %w", err)
	}

	// Handle version flag
	if *versionFlag {
		return &CommandArgs{Command: "version", Config: cfg}, nil
	}

	// Load configuration using the same flag set
	if err := cfg.LoadConfigWithFlagSet(fs); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return &CommandArgs{Command: "start", Config: cfg}, nil
}

// Execute runs the specified command
func (c *CLI) Execute(cmdArgs *CommandArgs) error {
	switch cmdArgs.Command {
	case "version":
		version.ShowVersion()
		return nil
	case "start":
		return c.cmdStart(cmdArgs.Config)
	default:
		return fmt.Errorf("unknown command: %s", cmdArgs.Command)
	}
}

// cmdStart starts the API server
func (c *CLI) cmdStart(cfg *api.Config) error {
	srv, err := api.NewServer(cfg)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
	defer srv.Close() //nolint:errcheck

	if err := srv.Start(); err != nil {
		return fmt.Errorf("server failed: %w", err)
	}

	return nil
}

func main() {
	// Parse command line arguments
	cmdArgs, err := ParseArgs(os.Args[1:])
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	// Create CLI with parsed config
	cli := NewCLI(cmdArgs.Config, os.Stdout, os.Stderr)

	// Execute command
	if err := cli.Execute(cmdArgs); err != nil {
		log.Fatalf("Error: %v", err)
	}
}
