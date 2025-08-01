package status

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/larsks/airdancer/internal/cli"
	"github.com/larsks/airdancer/internal/mqtt"
	"github.com/larsks/display1306/v2/display"
	"github.com/larsks/display1306/v2/display/fakedriver"
)

// Handler implements the CLI handler for airdancer-status
type Handler struct {
	display       *display.Display
	mqttClient    *mqtt.Client
	displayActive bool
	lastActivity  time.Time
	activityMutex sync.RWMutex
}

// NewHandler creates a new Handler instance
func NewHandler(d *display.Display) *Handler {
	return &Handler{
		display:       d,
		displayActive: true,
		lastActivity:  time.Now(),
	}
}

// Start implements the CommandHandler interface
func (h *Handler) Start(config cli.Configurable) error {
	cfg := config.(*Config)

	// Initialize display if not already done
	if h.display == nil {
		var d *display.Display
		var err error

		if cfg.DryRun {
			// Use fake display driver
			fakeDriver := fakedriver.NewFakeSSD1306()
			d, err = display.NewDisplay().WithDriver(fakeDriver).Build()
			if err != nil {
				return fmt.Errorf("failed to initialize fake display: %w", err)
			}
		} else {
			// Use real display driver (default behavior)
			d, err = display.NewDisplay().Build()
			if err != nil {
				return fmt.Errorf("failed to initialize display: %w", err)
			}
		}
		h.display = d
	}

	// Initialize the display
	if err := h.display.Init(); err != nil {
		return fmt.Errorf("failed to initialize display: %w", err)
	}

	// Initialize MQTT client if configured
	if cfg.MqttServer != "" {
		mqttConfig := mqtt.Config{
			ServerURL: cfg.MqttServer,
			ClientID:  "airdancer-status",
			OnConnect: func(client *mqtt.Client) {
				// Subscribe to button events once connected
				if err := client.Subscribe("event/button/#", 0, h.handleButtonEvent); err != nil {
					log.Printf("Failed to subscribe to button events: %v", err)
				} else {
					log.Printf("Subscribed to button events on MQTT")
				}
			},
		}

		client, err := mqtt.NewClient(mqttConfig)
		if err != nil {
			log.Printf("Failed to initialize MQTT client: %v", err)
		} else {
			h.mqttClient = client
		}
	}

	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Cleanup function
	cleanup := func() {
		log.Println("Shutting down gracefully...")
		if h.mqttClient != nil {
			h.mqttClient.Disconnect(250)
		}
		h.display.ClearScreen() //nolint:errcheck
		h.display.Close()       //nolint:errcheck
	}

	// Handle shutdown signal in a separate goroutine
	go func() {
		<-sigChan
		log.Println("Received shutdown signal")
		cancel()
	}()

	// Ensure cleanup happens on any exit
	defer cleanup()

	title := "*** AIRDANCER ***"
	titleLen := len(title)
	count := 0
	lastUpdate := time.Time{}

	apiAddr := "???"
	switchAddr := "???"
	switchString := "???"

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Context canceled, exit gracefully
			return nil
		case <-ticker.C:
			var err error

			// Rotate title
			curTitle := title[count:titleLen] + title[0:count]
			count = (count + 1) % titleLen

			if lastUpdate.Add(cfg.UpdateInterval).Before(time.Now()) {
				// Get interface addresses
				apiAddr, err = getInterfaceAddress("wlapi")
				if err != nil {
					apiAddr = "???"
				}

				switchAddr, err = getInterfaceAddress("wlswitch")
				if err != nil {
					switchAddr = "???"
				}

				// Get switch status
				switchStatus := getSwitchStatus(cfg.ServerURL)
				switchString = switchStatusToString(switchStatus)

				lastUpdate = time.Now()
			}

			// Check if display should be active based on timeout
			shouldBeActive := h.shouldDisplayBeActive(cfg.DisplayTimeout, cfg.MqttServer)
			h.setDisplayActive(shouldBeActive)

			// Only update display if it should be active
			if shouldBeActive {
				// Update display with current status
				lines := []string{
					curTitle,
					fmt.Sprintf("WLA: %s", apiAddr),
					fmt.Sprintf("WLS: %s", switchAddr),
					fmt.Sprintf("SWI: %s", switchString),
				}

				if err := h.display.PrintLines(0, lines); err != nil {
					log.Printf("failed to print lines to display: %v", err)
				}

				if err := h.display.Update(); err != nil {
					log.Printf("failed to update display: %v", err)
				}
			}
		}
	}
}

// APIResponse represents the standard API response format
type APIResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// MultiSwitchResponse represents the response from /switch/all
type MultiSwitchResponse struct {
	Summary  bool                       `json:"summary"`
	State    string                     `json:"state"`
	Count    uint                       `json:"count"`
	Switches map[string]*SwitchResponse `json:"switches"`
	Groups   map[string]*GroupResponse  `json:"groups,omitempty"`
}

// SwitchResponse represents individual switch data
type SwitchResponse struct {
	State        string   `json:"state"`
	CurrentState bool     `json:"currentState"`
	Duration     *int     `json:"duration,omitempty"`
	Period       *float64 `json:"period,omitempty"`
	DutyCycle    *float64 `json:"dutyCycle,omitempty"`
}

// GroupResponse represents switch group data
type GroupResponse struct {
	Switches []string `json:"switches"`
	Summary  bool     `json:"summary"`
	State    string   `json:"state"`
}

// SwitchState represents the state of a switch
type SwitchState struct {
	CurrentState bool
	Disabled     bool
}

// getSwitchStatus contacts the API server and returns switch states
func getSwitchStatus(serverURL string) []SwitchState {
	url := fmt.Sprintf("%s/switch/all", serverURL)

	resp, err := http.Get(url)
	if err != nil {
		log.Printf("failed to contact API server: %v", err)
		return []SwitchState{}
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		log.Printf("API server returned status %d", resp.StatusCode)
		return []SwitchState{}
	}

	var apiResponse APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		log.Printf("failed to decode API response: %v", err)
		return []SwitchState{}
	}

	if apiResponse.Status != "ok" {
		log.Printf("API returned error status: %s", apiResponse.Message)
		return []SwitchState{}
	}

	// Convert data to MultiSwitchResponse
	dataBytes, err := json.Marshal(apiResponse.Data)
	if err != nil {
		log.Printf("failed to marshal API data: %v", err)
		return []SwitchState{}
	}

	var switchData MultiSwitchResponse
	if err := json.Unmarshal(dataBytes, &switchData); err != nil {
		log.Printf("failed to unmarshal switch data: %v", err)
		return []SwitchState{}
	}

	// Extract switch names and sort them for consistent ordering
	var switchNames []string
	for name := range switchData.Switches {
		switchNames = append(switchNames, name)
	}
	sort.Strings(switchNames)

	// Extract switch states in sorted order
	var states []SwitchState
	for _, name := range switchNames {
		switchResp := switchData.Switches[name]
		states = append(states, SwitchState{
			CurrentState: switchResp.CurrentState,
			Disabled:     switchResp.State == "disabled",
		})
	}

	return states
}

// switchStatusToString converts switch states to _/X/? format
func switchStatusToString(states []SwitchState) string {
	var result strings.Builder
	for _, state := range states {
		if state.Disabled {
			result.WriteString("?")
		} else if state.CurrentState {
			result.WriteString("X")
		} else {
			result.WriteString("_")
		}
	}
	return result.String()
}

// IPAddr represents the JSON structure from 'ip -j addr show'
type IPAddr struct {
	IfIndex  int        `json:"ifindex"`
	IfName   string     `json:"ifname"`
	Flags    []string   `json:"flags"`
	AddrInfo []AddrInfo `json:"addr_info"`
}

// AddrInfo represents address information from 'ip -j addr show'
type AddrInfo struct {
	Family    string `json:"family"`
	Local     string `json:"local"`
	Prefixlen int    `json:"prefixlen"`
	Scope     string `json:"scope"`
}

// getInterfaceAddress uses 'ip -j addr show' to get interface address
func getInterfaceAddress(interfaceName string) (string, error) {
	cmd := exec.Command("ip", "-j", "addr", "show", interfaceName)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to execute ip command: %w", err)
	}

	var interfaces []IPAddr
	err = json.Unmarshal(output, &interfaces)
	if err != nil {
		return "", fmt.Errorf("failed to parse JSON output: %w", err)
	}

	if len(interfaces) != 1 {
		return "", fmt.Errorf("interface %s not found", interfaceName)
	}

	// Find the first IPv4 address that's not loopback
	for _, addr := range interfaces[0].AddrInfo {
		if addr.Family == "inet" && addr.Scope != "host" {
			return addr.Local, nil
		}
	}

	return "", fmt.Errorf("no IPv4 address found for interface %s", interfaceName)
}

// handleButtonEvent processes incoming MQTT button events
func (h *Handler) handleButtonEvent(topic string, payload []byte) {
	log.Printf("Received button event on topic %s: %s", topic, string(payload))

	// Reset the activity timer
	h.activityMutex.Lock()
	h.lastActivity = time.Now()

	// If display was inactive, reactivate it
	if !h.displayActive {
		h.displayActive = true
		log.Printf("Display reactivated by button event")
	}
	h.activityMutex.Unlock()
}

// shouldDisplayBeActive returns true if the display should be active based on timeout
func (h *Handler) shouldDisplayBeActive(displayTimeout time.Duration, mqttServerConfig string) bool {
	if displayTimeout <= 0 {
		return true // Timeout disabled
	}

	h.activityMutex.RLock()
	defer h.activityMutex.RUnlock()

	// Only allow blanking if we have a working MQTT connection to unblank the screen
	// If no MQTT is configured OR MQTT is configured but not working, never blank
	if mqttServerConfig == "" || h.mqttClient == nil || !h.mqttClient.IsConnected() {
		return true
	}

	return time.Since(h.lastActivity) < displayTimeout
}

// setDisplayActive sets the display active/inactive state
func (h *Handler) setDisplayActive(active bool) {
	h.activityMutex.Lock()
	defer h.activityMutex.Unlock()

	if h.displayActive != active {
		h.displayActive = active
		if !active {
			log.Printf("Display blanked due to inactivity")
			h.display.ClearScreen() //nolint:errcheck
		} else {
			log.Printf("Display activated")
		}
	}
}
