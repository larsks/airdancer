package switchdrivers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/larsks/airdancer/internal/switchcollection"
)

// TasmotaConfig represents Tasmota driver configuration
type TasmotaConfig struct {
	Addresses []string `mapstructure:"addresses"`
	Timeout   int      `mapstructure:"timeout"` // in seconds
}

// TasmotaFactory implements Factory for Tasmota drivers
type TasmotaFactory struct{}

// CreateDriver creates a new Tasmota switch collection
func (f *TasmotaFactory) CreateDriver(config map[string]interface{}) (switchcollection.SwitchCollection, error) {
	cfg, err := f.parseConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Tasmota config: %w", err)
	}

	if len(cfg.Addresses) == 0 {
		return nil, fmt.Errorf("Tasmota driver requires at least one address")
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 5 // Default timeout of 5 seconds
	}

	return NewTasmotaSwitchCollection(cfg.Addresses, time.Duration(cfg.Timeout)*time.Second), nil
}

// ValidateConfig validates Tasmota configuration
func (f *TasmotaFactory) ValidateConfig(config map[string]interface{}) error {
	cfg, err := f.parseConfig(config)
	if err != nil {
		return err
	}

	if len(cfg.Addresses) == 0 {
		return fmt.Errorf("Tasmota driver requires at least one address")
	}

	// Validate that addresses are valid URLs
	for i, addr := range cfg.Addresses {
		if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
			addr = "http://" + addr
		}
		if _, err := url.Parse(addr); err != nil {
			return fmt.Errorf("invalid address at index %d: %s", i, err)
		}
	}

	return nil
}

// parseConfig converts map to TasmotaConfig struct
func (f *TasmotaFactory) parseConfig(config map[string]interface{}) (*TasmotaConfig, error) {
	cfg := &TasmotaConfig{}

	if addresses, ok := config["addresses"].([]interface{}); ok {
		cfg.Addresses = make([]string, len(addresses))
		for i, addr := range addresses {
			if addrStr, ok := addr.(string); ok {
				cfg.Addresses[i] = addrStr
			} else {
				return nil, fmt.Errorf("address %d is not a string", i)
			}
		}
	} else if addresses, ok := config["addresses"].([]string); ok {
		cfg.Addresses = addresses
	} else {
		return nil, fmt.Errorf("addresses configuration is required and must be a string array")
	}

	if timeout, ok := config["timeout"].(int); ok {
		cfg.Timeout = timeout
	}

	return cfg, nil
}

// TasmotaResponse represents the JSON response from Tasmota devices
type TasmotaResponse struct {
	Power string `json:"POWER"`
}

// TasmotaSwitch represents a single Tasmota switch
type TasmotaSwitch struct {
	address string
	client  *http.Client
}

// NewTasmotaSwitch creates a new Tasmota switch
func NewTasmotaSwitch(address string, timeout time.Duration) *TasmotaSwitch {
	// Ensure address has http:// prefix
	if !strings.HasPrefix(address, "http://") && !strings.HasPrefix(address, "https://") {
		address = "http://" + address
	}

	return &TasmotaSwitch{
		address: address,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// TurnOn turns the switch on
func (s *TasmotaSwitch) TurnOn() error {
	_, err := s.sendCommand("Power+ON")
	return err
}

// TurnOff turns the switch off
func (s *TasmotaSwitch) TurnOff() error {
	_, err := s.sendCommand("Power+OFF")
	return err
}

// GetState returns the current state of the switch
func (s *TasmotaSwitch) GetState() (bool, error) {
	resp, err := s.sendCommand("Power")
	if err != nil {
		return false, err
	}
	return resp.Power == "ON", nil
}

// String returns a string representation of the switch
func (s *TasmotaSwitch) String() string {
	return fmt.Sprintf("TasmotaSwitch(%s)", s.address)
}

// sendCommand sends a command to the Tasmota device and returns the response
func (s *TasmotaSwitch) sendCommand(command string) (*TasmotaResponse, error) {
	url := fmt.Sprintf("%s/cm?cmnd=%s", s.address, command)

	resp, err := s.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var tasmotaResp TasmotaResponse
	if err := json.Unmarshal(body, &tasmotaResp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return &tasmotaResp, nil
}

// TasmotaSwitchCollection represents a collection of Tasmota switches
type TasmotaSwitchCollection struct {
	switches []switchcollection.Switch
}

// NewTasmotaSwitchCollection creates a new Tasmota switch collection
func NewTasmotaSwitchCollection(addresses []string, timeout time.Duration) *TasmotaSwitchCollection {
	switches := make([]switchcollection.Switch, len(addresses))
	for i, addr := range addresses {
		switches[i] = NewTasmotaSwitch(addr, timeout)
	}

	return &TasmotaSwitchCollection{
		switches: switches,
	}
}

// TurnOn turns on all switches in the collection
func (c *TasmotaSwitchCollection) TurnOn() error {
	for _, sw := range c.switches {
		if err := sw.TurnOn(); err != nil {
			return err
		}
	}
	return nil
}

// TurnOff turns off all switches in the collection
func (c *TasmotaSwitchCollection) TurnOff() error {
	for _, sw := range c.switches {
		if err := sw.TurnOff(); err != nil {
			return err
		}
	}
	return nil
}

// GetState returns true if any switch is on
func (c *TasmotaSwitchCollection) GetState() (bool, error) {
	for _, sw := range c.switches {
		state, err := sw.GetState()
		if err != nil {
			return false, err
		}
		if state {
			return true, nil
		}
	}
	return false, nil
}

// String returns a string representation of the collection
func (c *TasmotaSwitchCollection) String() string {
	return fmt.Sprintf("TasmotaSwitchCollection(%d switches)", len(c.switches))
}

// CountSwitches returns the number of switches in the collection
func (c *TasmotaSwitchCollection) CountSwitches() uint {
	return uint(len(c.switches))
}

// ListSwitches returns all switches in the collection
func (c *TasmotaSwitchCollection) ListSwitches() []switchcollection.Switch {
	return c.switches
}

// GetSwitch returns the switch at the specified index
func (c *TasmotaSwitchCollection) GetSwitch(id uint) (switchcollection.Switch, error) {
	if id >= uint(len(c.switches)) {
		return nil, fmt.Errorf("switch index %d out of range (have %d switches)", id, len(c.switches))
	}
	return c.switches[id], nil
}

// GetDetailedState returns the state of each switch in the collection
func (c *TasmotaSwitchCollection) GetDetailedState() ([]bool, error) {
	states := make([]bool, len(c.switches))
	for i, sw := range c.switches {
		state, err := sw.GetState()
		if err != nil {
			return nil, err
		}
		states[i] = state
	}
	return states, nil
}

// Init initializes the switch collection (no-op for Tasmota)
func (c *TasmotaSwitchCollection) Init() error {
	return nil
}

// Close closes the switch collection (no-op for Tasmota)
func (c *TasmotaSwitchCollection) Close() error {
	return nil
}

func init() {
	Register("tasmota", &TasmotaFactory{})
}
