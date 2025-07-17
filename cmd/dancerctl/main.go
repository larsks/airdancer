package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/larsks/airdancer/internal/config"
	_ "github.com/larsks/airdancer/internal/logsetup"
	"github.com/larsks/airdancer/internal/version"
	"github.com/spf13/pflag"
)

const (
	defaultServerURL = "http://localhost:8080"
)

type APIResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

type SwitchResponse struct {
	State        string   `json:"state"`
	CurrentState bool     `json:"currentState"`
	Duration     *uint    `json:"duration,omitempty"`
	Period       *float64 `json:"period,omitempty"`
	DutyCycle    *float64 `json:"dutyCycle,omitempty"`
}

type MultiSwitchResponse struct {
	Summary  bool                       `json:"summary"`
	State    string                     `json:"state"`
	Count    uint                       `json:"count"`
	Switches map[string]*SwitchResponse `json:"switches"`
	Groups   map[string]*GroupResponse  `json:"groups,omitempty"`
}

type GroupResponse struct {
	Switches []string `json:"switches"`
	Summary  bool     `json:"summary"`
	State    string   `json:"state"`
}

type SwitchRequest struct {
	State     string   `json:"state"`
	Duration  *uint    `json:"duration,omitempty"`
	Period    *float64 `json:"period,omitempty"`
	DutyCycle *float64 `json:"dutyCycle,omitempty"`
}

type Config struct {
	ServerURL          string `mapstructure:"server-url"`
	ConfigFile         string `mapstructure:"config-file"`
	explicitConfigFile bool   // Track if config file was explicitly set
}

func getDefaultConfigFile() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".config", "dancer", "dancer.toml")
}

func NewConfig() *Config {
	return &Config{
		ServerURL: defaultServerURL,
	}
}

func (c *Config) AddFlags(fs *pflag.FlagSet) {
	defaultConfigFile := getDefaultConfigFile()
	fs.StringVar(&c.ConfigFile, "config", defaultConfigFile, "Config file to use")
	fs.StringVar(&c.ServerURL, "server-url", c.ServerURL, "API server URL")
}

func (c *Config) LoadConfig() error {
	return c.LoadConfigWithFlagSet(pflag.CommandLine)
}

func (c *Config) LoadConfigWithFlagSet(fs *pflag.FlagSet) error {
	// Check if config file was explicitly set by comparing with default
	defaultConfigFile := getDefaultConfigFile()
	c.explicitConfigFile = c.ConfigFile != defaultConfigFile

	loader := config.NewConfigLoader()

	// If using default config file, check if it exists and only set if it does
	if !c.explicitConfigFile {
		if _, err := os.Stat(c.ConfigFile); os.IsNotExist(err) {
			// Default config file doesn't exist, don't try to load it
			c.ConfigFile = ""
		}
	} else {
		// Explicit config file was specified, check if it exists
		if _, err := os.Stat(c.ConfigFile); os.IsNotExist(err) {
			return fmt.Errorf("config file not found: %s", c.ConfigFile)
		}
	}

	loader.SetConfigFile(c.ConfigFile)

	loader.SetDefaults(map[string]any{
		"server-url": defaultServerURL,
	})

	return loader.LoadConfigWithFlagSet(c, fs)
}

// HTTPClient interface for testing
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// CLI represents the command line interface
type CLI struct {
	config     *Config
	httpClient HTTPClient
	stdout     io.Writer
	stderr     io.Writer
}

// NewCLI creates a new CLI instance
func NewCLI(cfg *Config, httpClient HTTPClient, stdout, stderr io.Writer) *CLI {
	return &CLI{
		config:     cfg,
		httpClient: httpClient,
		stdout:     stdout,
		stderr:     stderr,
	}
}

// CommandArgs represents parsed command line arguments
type CommandArgs struct {
	Command   string
	Args      []string
	Period    float64
	Duration  uint
	DutyCycle float64
	Config    *Config
}

// ParseArgs parses command line arguments using pflag.CommandLine
func ParseArgs(args []string) (*CommandArgs, error) {
	return ParseArgsWithFlagSet(args, pflag.CommandLine)
}

// ParseArgsWithFlagSet parses command line arguments with a custom flag set (for testing)
func ParseArgsWithFlagSet(args []string, fs *pflag.FlagSet) (*CommandArgs, error) {
	// Define flags
	versionFlag := fs.Bool("version", false, "Show version and exit")
	helpFlag := fs.BoolP("help", "h", false, "Show help")

	// Config flags
	cfg := NewConfig()
	cfg.AddFlags(fs)

	// Command-specific flags
	period := fs.Float64P("period", "p", 1.0, "Period in seconds (for blink/flipflop)")
	duration := fs.UintP("duration", "d", 0, "Duration in seconds (0 = indefinite)")
	dutyCycle := fs.Float64P("duty-cycle", "c", 0.5, "Duty cycle (0.0 to 1.0)")

	// Parse arguments
	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("failed to parse flags: %w", err)
	}

	// Handle version flag
	if *versionFlag {
		return &CommandArgs{Command: "version", Config: cfg}, nil
	}

	// Handle help flag
	if *helpFlag {
		return &CommandArgs{Command: "help", Config: cfg}, nil
	}

	// Get command and arguments
	remainingArgs := fs.Args()
	if len(remainingArgs) == 0 {
		return &CommandArgs{Command: "help", Config: cfg}, nil
	}

	// Load configuration using the same flag set
	if err := cfg.LoadConfigWithFlagSet(fs); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return &CommandArgs{
		Command:   remainingArgs[0],
		Args:      remainingArgs[1:],
		Period:    *period,
		Duration:  *duration,
		DutyCycle: *dutyCycle,
		Config:    cfg,
	}, nil
}

// Execute runs the specified command
func (c *CLI) Execute(cmdArgs *CommandArgs) error {
	switch cmdArgs.Command {
	case "version":
		version.ShowVersion()
		return nil
	case "help":
		c.showHelp()
		return nil
	case "switches":
		return c.cmdSwitches(cmdArgs.Args)
	case "blink":
		return c.cmdBlink(cmdArgs.Args, cmdArgs.Period, cmdArgs.Duration, cmdArgs.DutyCycle)
	case "flipflop":
		return c.cmdFlipflop(cmdArgs.Args, cmdArgs.Period, cmdArgs.Duration, cmdArgs.DutyCycle)
	case "on":
		return c.cmdOn(cmdArgs.Args, cmdArgs.Duration)
	case "off":
		return c.cmdOff(cmdArgs.Args, cmdArgs.Duration)
	case "toggle":
		return c.cmdToggle(cmdArgs.Args)
	case "status":
		return c.cmdStatus(cmdArgs.Args)
	default:
		return fmt.Errorf("unknown command: %s", cmdArgs.Command)
	}
}

func (c *CLI) showHelp() {
	//nolint:errcheck
	fmt.Fprintf(c.stdout, `dancerctl - Command line tool for controlling airdancer switches

Usage: dancerctl [flags] <command> [arguments]

Commands:
  switches                    List all switches
  blink <switch>              Blink a switch
  flipflop <switch_or_group>  Flipflop a switch group
  on <switch>                 Turn on a switch
  off <switch>                Turn off a switch
  toggle <switch>             Toggle a switch
  status <switch>             Get status of a switch
  help                        Show this help
  version                     Show version information

Flags:
  --config string       Config file to use (default "%s")
  -d, --duration uint   Duration in seconds (0 = indefinite)
  -c, --duty-cycle float Duty cycle (0.0 to 1.0) (default 0.5)
  -h, --help            Show help
  -p, --period float    Period in seconds (for blink/flipflop) (default 1)
  --server-url string   API server URL (default "%s")
  --version             Show version and exit
`, getDefaultConfigFile(), defaultServerURL)
}

func (c *CLI) cmdSwitches(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("switches command takes no arguments")
	}

	resp, err := c.makeAPIRequest("GET", "/switch/all", nil)
	if err != nil {
		return err
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return fmt.Errorf("error parsing response: %w", err)
	}

	if apiResp.Status != "ok" {
		return fmt.Errorf("API error: %s", apiResp.Message)
	}

	dataBytes, err := json.Marshal(apiResp.Data)
	if err != nil {
		return fmt.Errorf("error marshaling data: %w", err)
	}

	var multiResp MultiSwitchResponse
	if err := json.Unmarshal(dataBytes, &multiResp); err != nil {
		return fmt.Errorf("error parsing switch data: %w", err)
	}

	fmt.Fprintf(c.stdout, "Switches (%d total):\n", multiResp.Count) //nolint:errcheck
	for name, sw := range multiResp.Switches {
		status := "off"
		if sw.CurrentState {
			status = "on"
		}
		fmt.Fprintf(c.stdout, "  %s: %s (state: %s)\n", name, status, sw.State) //nolint:errcheck
	}

	if len(multiResp.Groups) > 0 {
		fmt.Fprintf(c.stdout, "\nGroups:\n") //nolint:errcheck
		for name, group := range multiResp.Groups {
			status := "off"
			if group.Summary {
				status = "on"
			}
			fmt.Fprintf(c.stdout, "  %s: %s (state: %s, switches: %s)\n", name, status, group.State, strings.Join(group.Switches, ", ")) //nolint:errcheck
		}
	}

	return nil
}

func (c *CLI) cmdBlink(args []string, period float64, duration uint, dutyCycle float64) error {
	if len(args) != 1 {
		return fmt.Errorf("blink command requires exactly one switch argument")
	}

	switchName := args[0]
	req := SwitchRequest{State: "blink"}

	if period > 0 {
		req.Period = &period
	}
	if duration > 0 {
		req.Duration = &duration
	}
	if dutyCycle > 0 {
		req.DutyCycle = &dutyCycle
	}

	if err := c.sendSwitchRequest(switchName, req); err != nil {
		return err
	}

	fmt.Fprintf(c.stdout, "Blink started for switch: %s\n", switchName) //nolint:errcheck
	return nil
}

func (c *CLI) cmdFlipflop(args []string, period float64, duration uint, dutyCycle float64) error {
	if len(args) != 1 {
		return fmt.Errorf("flipflop command requires exactly one switch/group argument")
	}

	switchName := args[0]
	req := SwitchRequest{State: "flipflop"}

	if period > 0 {
		req.Period = &period
	}
	if duration > 0 {
		req.Duration = &duration
	}
	if dutyCycle > 0 {
		req.DutyCycle = &dutyCycle
	}

	if err := c.sendSwitchRequest(switchName, req); err != nil {
		return err
	}

	fmt.Fprintf(c.stdout, "Flipflop started for switch/group: %s\n", switchName) //nolint:errcheck
	return nil
}

func (c *CLI) cmdOn(args []string, duration uint) error {
	if len(args) != 1 {
		return fmt.Errorf("on command requires exactly one switch argument")
	}

	switchName := args[0]
	req := SwitchRequest{State: "on"}

	if duration > 0 {
		req.Duration = &duration
	}

	if err := c.sendSwitchRequest(switchName, req); err != nil {
		return err
	}

	fmt.Fprintf(c.stdout, "Switch turned on: %s\n", switchName) //nolint:errcheck
	return nil
}

func (c *CLI) cmdOff(args []string, duration uint) error {
	if len(args) != 1 {
		return fmt.Errorf("off command requires exactly one switch argument")
	}

	switchName := args[0]
	req := SwitchRequest{State: "off"}

	if duration > 0 {
		req.Duration = &duration
	}

	if err := c.sendSwitchRequest(switchName, req); err != nil {
		return err
	}

	fmt.Fprintf(c.stdout, "Switch turned off: %s\n", switchName) //nolint:errcheck
	return nil
}

func (c *CLI) cmdToggle(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("toggle command requires exactly one switch argument")
	}

	switchName := args[0]
	req := SwitchRequest{State: "toggle"}

	if err := c.sendSwitchRequest(switchName, req); err != nil {
		return err
	}

	fmt.Fprintf(c.stdout, "Switch toggled: %s\n", switchName) //nolint:errcheck
	return nil
}

func (c *CLI) cmdStatus(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("status command requires exactly one switch argument")
	}

	switchName := args[0]

	resp, err := c.makeAPIRequest("GET", "/switch/"+switchName, nil)
	if err != nil {
		return err
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return fmt.Errorf("error parsing response: %w", err)
	}

	if apiResp.Status != "ok" {
		return fmt.Errorf("API error: %s", apiResp.Message)
	}

	dataBytes, err := json.Marshal(apiResp.Data)
	if err != nil {
		return fmt.Errorf("error marshaling data: %w", err)
	}

	// Try to parse as SwitchResponse first (for individual switches)
	var switchResp SwitchResponse
	if err := json.Unmarshal(dataBytes, &switchResp); err == nil {
		status := "off"
		if switchResp.CurrentState {
			status = "on"
		}
		fmt.Fprintf(c.stdout, "Switch: %s\n", switchName)      //nolint:errcheck
		fmt.Fprintf(c.stdout, "Status: %s\n", status)          //nolint:errcheck
		fmt.Fprintf(c.stdout, "State: %s\n", switchResp.State) //nolint:errcheck

		if switchResp.Duration != nil {
			fmt.Fprintf(c.stdout, "Duration: %d seconds\n", *switchResp.Duration) //nolint:errcheck
		}
		if switchResp.Period != nil {
			fmt.Fprintf(c.stdout, "Period: %.2f seconds\n", *switchResp.Period) //nolint:errcheck
		}
		if switchResp.DutyCycle != nil {
			fmt.Fprintf(c.stdout, "Duty Cycle: %.2f\n", *switchResp.DutyCycle) //nolint:errcheck
		}
	} else {
		// Try to parse as MultiSwitchResponse (for groups or "all")
		var multiResp MultiSwitchResponse
		if err := json.Unmarshal(dataBytes, &multiResp); err != nil {
			return fmt.Errorf("error parsing switch data: %w", err)
		}

		fmt.Fprintf(c.stdout, "Switch/Group: %s\n", switchName)        //nolint:errcheck
		fmt.Fprintf(c.stdout, "Summary Status: %s\n", multiResp.State) //nolint:errcheck
		fmt.Fprintf(c.stdout, "Count: %d\n", multiResp.Count)          //nolint:errcheck

		if len(multiResp.Switches) > 0 {
			fmt.Fprintf(c.stdout, "Switches:\n") //nolint:errcheck
			for name, sw := range multiResp.Switches {
				status := "off"
				if sw.CurrentState {
					status = "on"
				}
				fmt.Fprintf(c.stdout, "  %s: %s (state: %s)\n", name, status, sw.State) //nolint:errcheck
			}
		}
	}

	return nil
}

func (c *CLI) sendSwitchRequest(switchName string, req SwitchRequest) error {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.makeAPIRequest("POST", "/switch/"+switchName, reqBody)
	if err != nil {
		return err
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if apiResp.Status != "ok" {
		return fmt.Errorf("API error: %s", apiResp.Message)
	}

	return nil
}

func (c *CLI) makeAPIRequest(method, path string, body []byte) ([]byte, error) {
	url := c.config.ServerURL + path

	var req *http.Request
	var err error

	if body != nil {
		req, err = http.NewRequest(method, url, strings.NewReader(string(body)))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequest(method, url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	// Read the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for HTTP errors and try to parse API error message
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Try to parse the error response as JSON to get the API error message
		var apiResp APIResponse
		if err := json.Unmarshal(respBody, &apiResp); err == nil && apiResp.Message != "" {
			return nil, fmt.Errorf("API error: %s", apiResp.Message)
		}
		// Fall back to HTTP status if we can't parse the error
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	return respBody, nil
}

func main() {
	// Parse command line arguments
	cmdArgs, err := ParseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err) //nolint:errcheck
		os.Exit(1)
	}

	// Create CLI with parsed config
	cli := NewCLI(cmdArgs.Config, &http.Client{}, os.Stdout, os.Stderr)

	// Execute command
	if err := cli.Execute(cmdArgs); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err) //nolint:errcheck
		os.Exit(1)
	}
}
