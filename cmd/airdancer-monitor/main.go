package main

import (
	"fmt"
	"io"
	"log"
	"os"

	flag "github.com/spf13/pflag"

	_ "github.com/larsks/airdancer/internal/logsetup"
	"github.com/larsks/airdancer/internal/monitor"
	"github.com/larsks/airdancer/internal/version"
)

// MonitorInterface abstracts the email monitor for testing
type MonitorInterface interface {
	Start()
}

// CLI represents the command line interface for airdancer-monitor
type CLI struct {
	config *monitor.Config
	stdout io.Writer
	stderr io.Writer
}

// NewCLI creates a new CLI instance
func NewCLI(cfg *monitor.Config, stdout, stderr io.Writer) *CLI {
	return &CLI{
		config: cfg,
		stdout: stdout,
		stderr: stderr,
	}
}

// CommandArgs represents parsed command line arguments
type CommandArgs struct {
	Command string
	Config  *monitor.Config
}

// ParseArgs parses command line arguments using pflag.CommandLine
func ParseArgs(args []string) (*CommandArgs, error) {
	return ParseArgsWithFlagSet(args, flag.CommandLine)
}

// ParseArgsWithFlagSet parses command line arguments with a custom flag set (for testing)
func ParseArgsWithFlagSet(args []string, fs *flag.FlagSet) (*CommandArgs, error) {
	// Define flags
	versionFlag := fs.Bool("version", false, "Show version and exit")

	// Config flags
	cfg := monitor.NewConfig()
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
	err := cfg.LoadConfigWithFlagSet(fs)
	if err != nil {
		// Only fail if config file was explicitly specified but couldn't be loaded
		if cfg.ConfigFile != "" {
			return nil, fmt.Errorf("failed to load config: %w", err)
		}
		// Log warning for default config issues but continue
		log.Printf("using default configuration: %v", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("configuration error: %w", err)
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

// cmdStart starts the email monitor
func (c *CLI) cmdStart(cfg *monitor.Config) error {
	// Create email monitor
	emailMonitor, err := monitor.NewEmailMonitorWithDefaults(*cfg)
	if err != nil {
		return fmt.Errorf("failed to create monitor: %w", err)
	}

	// Start monitoring (this blocks)
	emailMonitor.Start()

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
