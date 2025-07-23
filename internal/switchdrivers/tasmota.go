package switchdrivers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
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
		return nil, fmt.Errorf("tasmota driver requires at least one address")
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
		return fmt.Errorf("tasmota driver requires at least one address")
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
	address  string
	client   *http.Client
	disabled bool
	mutex    sync.RWMutex
}

// NewTasmotaSwitch creates a new Tasmota switch
func NewTasmotaSwitch(address string, timeout time.Duration) *TasmotaSwitch {
	// Ensure address has http:// prefix
	if !strings.HasPrefix(address, "http://") && !strings.HasPrefix(address, "https://") {
		address = "http://" + address
	}

	return &TasmotaSwitch{
		address:  address,
		disabled: false,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// TurnOn turns the switch on
func (s *TasmotaSwitch) TurnOn() error {
	s.mutex.RLock()
	if s.disabled {
		s.mutex.RUnlock()
		return fmt.Errorf("switch %s is disabled due to network issues", s.address)
	}
	s.mutex.RUnlock()

	_, err := s.sendCommand("Power+ON")
	if err != nil {
		s.markDisabled()
		return err
	}
	s.markEnabled()
	return nil
}

// TurnOff turns the switch off
func (s *TasmotaSwitch) TurnOff() error {
	s.mutex.RLock()
	if s.disabled {
		s.mutex.RUnlock()
		return fmt.Errorf("switch %s is disabled due to network issues", s.address)
	}
	s.mutex.RUnlock()

	_, err := s.sendCommand("Power+OFF")
	if err != nil {
		s.markDisabled()
		return err
	}
	s.markEnabled()
	return nil
}

// GetState returns the current state of the switch
func (s *TasmotaSwitch) GetState() (bool, error) {
	resp, err := s.sendCommand("Power")
	if err != nil {
		s.markDisabled()
		return false, err
	}
	s.markEnabled()
	return resp.Power == "ON", nil
}

// IsDisabled returns true if the switch is disabled due to network issues
func (s *TasmotaSwitch) IsDisabled() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.disabled
}

// markDisabled marks the switch as disabled
func (s *TasmotaSwitch) markDisabled() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if !s.disabled {
		s.disabled = true
		log.Printf("switch %s marked as disabled due to network connectivity issues", s.address)
	}
}

// markEnabled marks the switch as enabled
func (s *TasmotaSwitch) markEnabled() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.disabled {
		s.disabled = false
		log.Printf("switch %s re-enabled after network connectivity restored", s.address)
	}
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
	defer resp.Body.Close() //nolint:errcheck

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
	switches   []switchcollection.Switch
	cancelFunc context.CancelFunc
	monitorCtx context.Context
}

// NewTasmotaSwitchCollection creates a new Tasmota switch collection
func NewTasmotaSwitchCollection(addresses []string, timeout time.Duration) *TasmotaSwitchCollection {
	switches := make([]switchcollection.Switch, len(addresses))
	for i, addr := range addresses {
		switches[i] = NewTasmotaSwitch(addr, timeout)
	}

	ctx, cancel := context.WithCancel(context.Background())
	collection := &TasmotaSwitchCollection{
		switches:   switches,
		cancelFunc: cancel,
		monitorCtx: ctx,
	}

	// Start monitoring goroutine
	go collection.monitorSwitches()
	log.Printf("started background monitoring for Tasmota switches (checking every 30 seconds)")

	return collection
}

// TurnOn turns on all switches in the collection
func (c *TasmotaSwitchCollection) TurnOn() error {
	var errors []error
	for _, sw := range c.switches {
		if err := sw.TurnOn(); err != nil {
			errors = append(errors, err)
		}
	}
	if len(errors) > 0 {
		return fmt.Errorf("failed to turn on some switches: %v", errors)
	}
	return nil
}

// TurnOff turns off all switches in the collection
func (c *TasmotaSwitchCollection) TurnOff() error {
	var errors []error
	for _, sw := range c.switches {
		if err := sw.TurnOff(); err != nil {
			errors = append(errors, err)
		}
	}
	if len(errors) > 0 {
		return fmt.Errorf("failed to turn off some switches: %v", errors)
	}
	return nil
}

// GetState returns true if any switch is on
func (c *TasmotaSwitchCollection) GetState() (bool, error) {
	for _, sw := range c.switches {
		// Skip disabled switches when determining collection state
		if sw.IsDisabled() {
			continue
		}
		state, err := sw.GetState()
		if err != nil {
			continue // Skip switches with errors, they're likely disabled
		}
		if state {
			return true, nil
		}
	}
	return false, nil
}

// IsDisabled returns false since Tasmota switch collections are never disabled (individual switches can be)
func (c *TasmotaSwitchCollection) IsDisabled() bool {
	return false
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
		// For disabled switches, report them as off
		if sw.IsDisabled() {
			states[i] = false
			continue
		}
		state, err := sw.GetState()
		if err != nil {
			states[i] = false // Report as off if there's an error
			continue
		}
		states[i] = state
	}
	return states, nil
}

// Init initializes the switch collection and checks initial connectivity
func (c *TasmotaSwitchCollection) Init() error {
	log.Printf("initializing Tasmota switch collection with %d switches", len(c.switches))
	// Perform initial connectivity check for all switches
	for _, sw := range c.switches {
		if tasmotaSwitch, ok := sw.(*TasmotaSwitch); ok {
			// Try to get the initial state to test connectivity
			_, err := tasmotaSwitch.sendCommand("Power")
			if err != nil {
				// Mark as disabled if unreachable during initialization
				tasmotaSwitch.markDisabled()
			} else {
				log.Printf("switch %s is reachable and ready", tasmotaSwitch.address)
			}
		}
	}
	return nil
}

// Close closes the switch collection
func (c *TasmotaSwitchCollection) Close() error {
	if c.cancelFunc != nil {
		log.Printf("stopping Tasmota switch monitoring")
		c.cancelFunc()
	}
	return nil
}

// monitorSwitches periodically checks disabled switches to see if they come back online
func (c *TasmotaSwitchCollection) monitorSwitches() {
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-c.monitorCtx.Done():
			return
		case <-ticker.C:
			c.checkDisabledSwitches()
		}
	}
}

// checkDisabledSwitches attempts to re-enable disabled switches
func (c *TasmotaSwitchCollection) checkDisabledSwitches() {
	disabledCount := 0
	for _, sw := range c.switches {
		if tasmotaSwitch, ok := sw.(*TasmotaSwitch); ok && tasmotaSwitch.IsDisabled() {
			disabledCount++
			// Try to get state to test connectivity
			_, err := tasmotaSwitch.sendCommand("Power")
			if err == nil {
				// Switch is back online, mark as enabled
				tasmotaSwitch.markEnabled()
			}
		}
	}
	if disabledCount > 0 {
		log.Printf("monitoring check: %d disabled switches found", disabledCount)
	}
}

func init() {
	MustRegister("tasmota", &TasmotaFactory{})
}
