package main

import (
	"encoding/json"
	"fmt"
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

	return loader.LoadConfig(c)
}

var cfg *Config

func main() {
	// Define flags
	versionFlag := pflag.Bool("version", false, "Show version and exit")
	helpFlag := pflag.BoolP("help", "h", false, "Show help")

	// Config flags
	cfg = NewConfig()
	cfg.AddFlags(pflag.CommandLine)

	// Command-specific flags
	var (
		period    = pflag.Float64P("period", "p", 1.0, "Period in seconds (for blink/flipflop)")
		duration  = pflag.UintP("duration", "d", 0, "Duration in seconds (0 = indefinite)")
		dutyCycle = pflag.Float64P("duty-cycle", "c", 0.5, "Duty cycle (0.0 to 1.0)")
	)

	pflag.Parse()

	// Handle version flag
	if *versionFlag {
		version.ShowVersion()
		os.Exit(0)
	}

	// Load configuration
	if err := cfg.LoadConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Get command and arguments
	args := pflag.Args()
	if len(args) == 0 || *helpFlag {
		showHelp()
		os.Exit(0)
	}

	command := args[0]
	commandArgs := args[1:]

	// Execute command
	switch command {
	case "switches":
		cmdSwitches(commandArgs)
	case "blink":
		cmdBlink(commandArgs, *period, *duration, *dutyCycle)
	case "flipflop":
		cmdFlipflop(commandArgs, *period, *duration, *dutyCycle)
	case "on":
		cmdOn(commandArgs, *duration)
	case "off":
		cmdOff(commandArgs, *duration)
	case "toggle":
		cmdToggle(commandArgs)
	case "status":
		cmdStatus(commandArgs)
	case "help":
		showHelp()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		showHelp()
		os.Exit(1)
	}
}

func showHelp() {
	fmt.Printf(`dancerctl - Command line tool for controlling airdancer switches

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

Flags:
`)
	pflag.PrintDefaults()
}

func cmdSwitches(args []string) {
	if len(args) > 0 {
		fmt.Fprintf(os.Stderr, "switches command takes no arguments\n")
		os.Exit(1)
	}

	resp, err := makeAPIRequest("GET", "/switch/all", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing response: %v\n", err)
		os.Exit(1)
	}

	if apiResp.Status != "ok" {
		fmt.Fprintf(os.Stderr, "API error: %s\n", apiResp.Message)
		os.Exit(1)
	}

	dataBytes, err := json.Marshal(apiResp.Data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling data: %v\n", err)
		os.Exit(1)
	}

	var multiResp MultiSwitchResponse
	if err := json.Unmarshal(dataBytes, &multiResp); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing switch data: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Switches (%d total):\n", multiResp.Count)
	for name, sw := range multiResp.Switches {
		status := "off"
		if sw.CurrentState {
			status = "on"
		}
		fmt.Printf("  %s: %s (state: %s)\n", name, status, sw.State)
	}

	if len(multiResp.Groups) > 0 {
		fmt.Printf("\nGroups:\n")
		for name, group := range multiResp.Groups {
			status := "off"
			if group.Summary {
				status = "on"
			}
			fmt.Printf("  %s: %s (state: %s, switches: %s)\n", name, status, group.State, strings.Join(group.Switches, ", "))
		}
	}
}

func cmdBlink(args []string, period float64, duration uint, dutyCycle float64) {
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "blink command requires exactly one switch argument\n")
		os.Exit(1)
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

	if err := sendSwitchRequest(switchName, req); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Blink started for switch: %s\n", switchName)
}

func cmdFlipflop(args []string, period float64, duration uint, dutyCycle float64) {
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "flipflop command requires exactly one switch/group argument\n")
		os.Exit(1)
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

	if err := sendSwitchRequest(switchName, req); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Flipflop started for switch/group: %s\n", switchName)
}

func cmdOn(args []string, duration uint) {
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "on command requires exactly one switch argument\n")
		os.Exit(1)
	}

	switchName := args[0]
	req := SwitchRequest{State: "on"}

	if duration > 0 {
		req.Duration = &duration
	}

	if err := sendSwitchRequest(switchName, req); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Switch turned on: %s\n", switchName)
}

func cmdOff(args []string, duration uint) {
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "off command requires exactly one switch argument\n")
		os.Exit(1)
	}

	switchName := args[0]
	req := SwitchRequest{State: "off"}

	if duration > 0 {
		req.Duration = &duration
	}

	if err := sendSwitchRequest(switchName, req); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Switch turned off: %s\n", switchName)
}

func cmdToggle(args []string) {
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "toggle command requires exactly one switch argument\n")
		os.Exit(1)
	}

	switchName := args[0]
	req := SwitchRequest{State: "toggle"}

	if err := sendSwitchRequest(switchName, req); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Switch toggled: %s\n", switchName)
}

func cmdStatus(args []string) {
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "status command requires exactly one switch argument\n")
		os.Exit(1)
	}

	switchName := args[0]

	resp, err := makeAPIRequest("GET", "/switch/"+switchName, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing response: %v\n", err)
		os.Exit(1)
	}

	if apiResp.Status != "ok" {
		fmt.Fprintf(os.Stderr, "API error: %s\n", apiResp.Message)
		os.Exit(1)
	}

	dataBytes, err := json.Marshal(apiResp.Data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling data: %v\n", err)
		os.Exit(1)
	}

	// Try to parse as SwitchResponse first (for individual switches)
	var switchResp SwitchResponse
	if err := json.Unmarshal(dataBytes, &switchResp); err == nil {
		status := "off"
		if switchResp.CurrentState {
			status = "on"
		}
		fmt.Printf("Switch: %s\n", switchName)
		fmt.Printf("Status: %s\n", status)
		fmt.Printf("State: %s\n", switchResp.State)

		if switchResp.Duration != nil {
			fmt.Printf("Duration: %d seconds\n", *switchResp.Duration)
		}
		if switchResp.Period != nil {
			fmt.Printf("Period: %.2f seconds\n", *switchResp.Period)
		}
		if switchResp.DutyCycle != nil {
			fmt.Printf("Duty Cycle: %.2f\n", *switchResp.DutyCycle)
		}
	} else {
		// Try to parse as MultiSwitchResponse (for groups or "all")
		var multiResp MultiSwitchResponse
		if err := json.Unmarshal(dataBytes, &multiResp); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing switch data: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Switch/Group: %s\n", switchName)
		fmt.Printf("Summary Status: %s\n", multiResp.State)
		fmt.Printf("Count: %d\n", multiResp.Count)

		if len(multiResp.Switches) > 0 {
			fmt.Printf("Switches:\n")
			for name, sw := range multiResp.Switches {
				status := "off"
				if sw.CurrentState {
					status = "on"
				}
				fmt.Printf("  %s: %s (state: %s)\n", name, status, sw.State)
			}
		}
	}
}

func sendSwitchRequest(switchName string, req SwitchRequest) error {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := makeAPIRequest("POST", "/switch/"+switchName, reqBody)
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

func makeAPIRequest(method, path string, body []byte) ([]byte, error) {
	url := cfg.ServerURL + path

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

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	// Read the response body first
	respBody := make([]byte, 0)
	buf := make([]byte, 1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			respBody = append(respBody, buf[:n]...)
		}
		if err != nil {
			break
		}
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

