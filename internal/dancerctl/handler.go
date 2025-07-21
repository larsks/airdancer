package dancerctl

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/larsks/airdancer/internal/cli"
	"github.com/larsks/airdancer/internal/version"
	"github.com/spf13/pflag"
)

// APIResponse represents the standard API response format
type APIResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// SwitchResponse represents a single switch status response
type SwitchResponse struct {
	State        string   `json:"state"`
	CurrentState bool     `json:"currentState"`
	Duration     *uint    `json:"duration,omitempty"`
	Period       *float64 `json:"period,omitempty"`
	DutyCycle    *float64 `json:"dutyCycle,omitempty"`
}

// MultiSwitchResponse represents a multi-switch status response
type MultiSwitchResponse struct {
	Summary  bool                       `json:"summary"`
	State    string                     `json:"state"`
	Count    uint                       `json:"count"`
	Switches map[string]*SwitchResponse `json:"switches"`
	Groups   map[string]*GroupResponse  `json:"groups,omitempty"`
}

// GroupResponse represents a switch group response
type GroupResponse struct {
	Switches []string `json:"switches"`
	Summary  bool     `json:"summary"`
	State    string   `json:"state"`
}

// SwitchRequest represents a request to control a switch
type SwitchRequest struct {
	State     string   `json:"state"`
	Duration  *uint    `json:"duration,omitempty"`
	Period    *float64 `json:"period,omitempty"`
	DutyCycle *float64 `json:"dutyCycle,omitempty"`
}

// HTTPClient interface for testing
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Handler implements the dancerctl command handler
type Handler struct {
	config     *Config
	httpClient HTTPClient
	stdout     io.Writer
	stderr     io.Writer

	// Command-specific flags
	period    float64
	duration  uint
	dutyCycle float64
}

// NewHandler creates a new dancerctl handler
func NewHandler() *Handler {
	return &Handler{
		httpClient: &http.Client{},
		stdout:     os.Stdout,
		stderr:     os.Stderr,
	}
}

// AddFlags adds command-specific flags
func (h *Handler) AddFlags(fs *pflag.FlagSet) {
	fs.Float64VarP(&h.period, "period", "p", 1.0, "Period in seconds (for blink/flipflop)")
	fs.UintVarP(&h.duration, "duration", "d", 0, "Duration in seconds (0 = indefinite)")
	fs.Float64VarP(&h.dutyCycle, "duty-cycle", "c", 0.5, "Duty cycle (0.0 to 1.0)")
}

// Execute implements the cli.SubCommandHandler interface
func (h *Handler) Execute(cmdArgs *cli.CommandArgs) error {
	h.config = cmdArgs.Config.(*Config)

	// Handle special commands first
	if cmdArgs.Command == "help" || len(cmdArgs.Args) == 0 {
		h.showHelp()
		return nil
	}

	command := cmdArgs.Args[0]
	args := cmdArgs.Args[1:]

	switch command {
	case "version":
		version.ShowVersion()
		return nil
	case "help":
		h.showHelp()
		return nil
	case "blink":
		return h.cmdBlink(args)
	case "flipflop":
		return h.cmdFlipflop(args)
	case "on":
		return h.cmdOn(args)
	case "off":
		return h.cmdOff(args)
	case "toggle":
		return h.cmdToggle(args)
	case "status":
		return h.cmdStatus(args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func (h *Handler) showHelp() {
	fmt.Fprintf(h.stdout, `dancerctl - Command line tool for controlling airdancer switches

Usage: dancerctl [flags] <command> [arguments]

Commands:
  blink <switch>              Blink a switch
  flipflop <switch_or_group>  Flipflop a switch group
  on <switch>                 Turn on a switch
  off <switch>                Turn off a switch
  toggle <switch>             Toggle a switch
  status [switch]             Get status of a switch or list all switches
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

func (h *Handler) cmdBlink(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("blink command requires exactly one switch argument")
	}

	switchName := args[0]
	req := SwitchRequest{State: "blink"}

	if h.period > 0 {
		req.Period = &h.period
	}
	if h.duration > 0 {
		req.Duration = &h.duration
	}
	if h.dutyCycle > 0 {
		req.DutyCycle = &h.dutyCycle
	}

	if err := h.sendSwitchRequest(switchName, req); err != nil {
		return err
	}

	fmt.Fprintf(h.stdout, "Blink started for switch: %s\n", switchName)
	return nil
}

func (h *Handler) cmdFlipflop(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("flipflop command requires exactly one switch/group argument")
	}

	switchName := args[0]
	req := SwitchRequest{State: "flipflop"}

	if h.period > 0 {
		req.Period = &h.period
	}
	if h.duration > 0 {
		req.Duration = &h.duration
	}
	if h.dutyCycle > 0 {
		req.DutyCycle = &h.dutyCycle
	}

	if err := h.sendSwitchRequest(switchName, req); err != nil {
		return err
	}

	fmt.Fprintf(h.stdout, "Flipflop started for switch/group: %s\n", switchName)
	return nil
}

func (h *Handler) cmdOn(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("on command requires exactly one switch argument")
	}

	switchName := args[0]
	req := SwitchRequest{State: "on"}

	if h.duration > 0 {
		req.Duration = &h.duration
	}

	if err := h.sendSwitchRequest(switchName, req); err != nil {
		return err
	}

	fmt.Fprintf(h.stdout, "Switch turned on: %s\n", switchName)
	return nil
}

func (h *Handler) cmdOff(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("off command requires exactly one switch argument")
	}

	switchName := args[0]
	req := SwitchRequest{State: "off"}

	if h.duration > 0 {
		req.Duration = &h.duration
	}

	if err := h.sendSwitchRequest(switchName, req); err != nil {
		return err
	}

	fmt.Fprintf(h.stdout, "Switch turned off: %s\n", switchName)
	return nil
}

func (h *Handler) cmdToggle(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("toggle command requires exactly one switch argument")
	}

	switchName := args[0]
	req := SwitchRequest{State: "toggle"}

	if err := h.sendSwitchRequest(switchName, req); err != nil {
		return err
	}

	fmt.Fprintf(h.stdout, "Switch toggled: %s\n", switchName)
	return nil
}

func (h *Handler) cmdStatus(args []string) error {
	if len(args) == 0 {
		return h.cmdSwitches()
	}

	if len(args) != 1 {
		return fmt.Errorf("status command requires zero or one switch argument")
	}

	switchName := args[0]

	resp, err := h.makeAPIRequest("GET", "/switch/"+switchName, nil)
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
		fmt.Fprintf(h.stdout, "Switch: %s\n", switchName)
		fmt.Fprintf(h.stdout, "Status: %s\n", status)
		fmt.Fprintf(h.stdout, "State: %s\n", switchResp.State)

		if switchResp.Duration != nil {
			fmt.Fprintf(h.stdout, "Duration: %d seconds\n", *switchResp.Duration)
		}
		if switchResp.Period != nil {
			fmt.Fprintf(h.stdout, "Period: %.2f seconds\n", *switchResp.Period)
		}
		if switchResp.DutyCycle != nil {
			fmt.Fprintf(h.stdout, "Duty Cycle: %.2f\n", *switchResp.DutyCycle)
		}
	} else {
		// Try to parse as MultiSwitchResponse (for groups or "all")
		var multiResp MultiSwitchResponse
		if err := json.Unmarshal(dataBytes, &multiResp); err != nil {
			return fmt.Errorf("error parsing switch data: %w", err)
		}

		fmt.Fprintf(h.stdout, "Switch/Group: %s\n", switchName)
		fmt.Fprintf(h.stdout, "Summary Status: %s\n", multiResp.State)
		fmt.Fprintf(h.stdout, "Count: %d\n", multiResp.Count)

		if len(multiResp.Switches) > 0 {
			fmt.Fprintf(h.stdout, "Switches:\n")
			for name, sw := range multiResp.Switches {
				status := "off"
				if sw.CurrentState {
					status = "on"
				}
				fmt.Fprintf(h.stdout, "  %s: %s (state: %s)\n", name, status, sw.State)
			}
		}
	}

	return nil
}

func (h *Handler) cmdSwitches() error {
	resp, err := h.makeAPIRequest("GET", "/switch/all", nil)
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

	fmt.Fprintf(h.stdout, "Switches (%d total):\n", multiResp.Count)
	for name, sw := range multiResp.Switches {
		status := "off"
		if sw.CurrentState {
			status = "on"
		}
		fmt.Fprintf(h.stdout, "  %s: %s (state: %s)\n", name, status, sw.State)
	}

	if len(multiResp.Groups) > 0 {
		fmt.Fprintf(h.stdout, "\nGroups:\n")
		for name, group := range multiResp.Groups {
			status := "off"
			if group.Summary {
				status = "on"
			}
			fmt.Fprintf(h.stdout, "  %s: %s (state: %s, switches: %s)\n", name, status, group.State, strings.Join(group.Switches, ", "))
		}
	}

	return nil
}

func (h *Handler) sendSwitchRequest(switchName string, req SwitchRequest) error {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := h.makeAPIRequest("POST", "/switch/"+switchName, reqBody)
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

func (h *Handler) makeAPIRequest(method, path string, body []byte) ([]byte, error) {
	url := h.config.ServerURL + path

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

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

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
