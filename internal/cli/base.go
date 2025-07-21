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

// SubCommandHandler represents a command handler that processes subcommands and arguments
type SubCommandHandler interface {
	Execute(cmdArgs *CommandArgs) error
	AddFlags(fs *pflag.FlagSet)
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
	Args    []string
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

// ParseArgsWithSubCommands provides argument parsing for commands with subcommands (like dancerctl)
func (c *BaseCLI) ParseArgsWithSubCommands(args []string, configFactory func() Configurable, handler SubCommandHandler) (*CommandArgs, error) {
	return c.ParseArgsWithSubCommandsAndFlagSet(args, configFactory, handler, pflag.CommandLine)
}

// ParseArgsWithSubCommandsAndFlagSet provides argument parsing with subcommands using a custom flag set
func (c *BaseCLI) ParseArgsWithSubCommandsAndFlagSet(args []string, configFactory func() Configurable, handler SubCommandHandler, fs *pflag.FlagSet) (*CommandArgs, error) {
	// Define standard flags
	versionFlag := fs.Bool("version", false, "Show version and exit")
	helpFlag := fs.BoolP("help", "h", false, "Show help")

	// Create config and add its flags
	cfg := configFactory()
	cfg.AddFlags(fs)

	// Add handler-specific flags
	handler.AddFlags(fs)

	// Parse arguments
	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("failed to parse flags: %w", err)
	}

	// Handle version flag
	if *versionFlag {
		return &CommandArgs{Command: "version", Args: []string{}, Config: cfg}, nil
	}

	// Handle help flag or no arguments
	remainingArgs := fs.Args()
	if *helpFlag || len(remainingArgs) == 0 {
		return &CommandArgs{Command: "help", Args: []string{}, Config: cfg}, nil
	}

	// Load configuration using the same flag set
	if err := cfg.LoadConfigWithFlagSet(fs); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return &CommandArgs{
		Command: "execute",
		Args:    remainingArgs,
		Config:  cfg,
	}, nil
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

// ExecuteWithSubCommands runs the specified command using subcommand patterns
func (c *BaseCLI) ExecuteWithSubCommands(cmdArgs *CommandArgs, handler SubCommandHandler) error {
	switch cmdArgs.Command {
	case "version":
		version.ShowVersion()
		return nil
	case "help", "execute":
		return handler.Execute(cmdArgs)
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

// SubCommandMain provides a complete main function implementation for CLI tools with subcommands
func SubCommandMain(configFactory func() Configurable, handler SubCommandHandler) {
	cli := NewBaseCLI(os.Stdout, os.Stderr)

	// Parse command line arguments
	cmdArgs, err := cli.ParseArgsWithSubCommands(os.Args[1:], configFactory, handler)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	// Execute command
	if err := cli.ExecuteWithSubCommands(cmdArgs, handler); err != nil {
		log.Fatalf("Error: %v", err)
	}
}
