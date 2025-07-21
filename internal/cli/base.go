package cli

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/larsks/airdancer/internal/version"
	"github.com/spf13/pflag"
)

// Configurable represents a type that can be configured via flags and config files
type Configurable interface {
	AddFlags(fs *pflag.FlagSet)
	LoadConfigWithFlagSet(fs *pflag.FlagSet) error
}

// CommandHandler represents a command that can be executed
type CommandHandler interface {
	Start(config Configurable) error
}

// BaseCLI provides common CLI functionality
type BaseCLI struct {
	stdout io.Writer
	stderr io.Writer
}

// NewBaseCLI creates a new BaseCLI instance
func NewBaseCLI(stdout, stderr io.Writer) *BaseCLI {
	return &BaseCLI{
		stdout: stdout,
		stderr: stderr,
	}
}

// CommandArgs represents parsed command line arguments
type CommandArgs struct {
	Command string
	Config  Configurable
}

// ParseArgsStandard provides standard argument parsing for version/help/start commands
func (c *BaseCLI) ParseArgsStandard(args []string, configFactory func() Configurable) (*CommandArgs, error) {
	return c.ParseArgsStandardWithFlagSet(args, configFactory, pflag.CommandLine)
}

// ParseArgsStandardWithFlagSet provides standard argument parsing with a custom flag set
func (c *BaseCLI) ParseArgsStandardWithFlagSet(args []string, configFactory func() Configurable, fs *pflag.FlagSet) (*CommandArgs, error) {
	// Define standard flags
	versionFlag := fs.Bool("version", false, "Show version and exit")

	// Create config and add its flags
	cfg := configFactory()
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

// Execute runs the specified command using standard patterns
func (c *BaseCLI) Execute(cmdArgs *CommandArgs, handler CommandHandler) error {
	switch cmdArgs.Command {
	case "version":
		version.ShowVersion()
		return nil
	case "start":
		return handler.Start(cmdArgs.Config)
	default:
		return fmt.Errorf("unknown command: %s", cmdArgs.Command)
	}
}

// StandardMain provides a complete main function implementation for simple services
func StandardMain(configFactory func() Configurable, handler CommandHandler) {
	cli := NewBaseCLI(os.Stdout, os.Stderr)

	// Parse command line arguments
	cmdArgs, err := cli.ParseArgsStandard(os.Args[1:], configFactory)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	// Execute command
	if err := cli.Execute(cmdArgs, handler); err != nil {
		log.Fatalf("Error: %v", err)
	}
}
