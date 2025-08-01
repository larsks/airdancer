package status

import (
	"testing"
	"time"

	"github.com/larsks/display1306/v2/display"
	"github.com/larsks/display1306/v2/display/fakedriver"
)

func TestNewHandler(t *testing.T) {
	// Create a fake display for testing
	d, err := display.NewDisplay().WithDriver(fakedriver.NewFakeSSD1306()).Build()
	if err != nil {
		t.Fatalf("Failed to build fake display: %v", err)
	}

	handler := NewHandler(d)
	if handler == nil {
		t.Fatal("NewHandler returned nil")
	}
	if handler.display == nil {
		t.Fatal("Handler display is nil")
	}
}

func TestHandlerWithFakeDisplay(t *testing.T) {
	// Create display with fake driver and build it
	d, err := display.NewDisplay().WithDriver(fakedriver.NewFakeSSD1306()).Build()
	if err != nil {
		t.Fatalf("Failed to build display: %v", err)
	}

	handler := NewHandler(d)
	if handler == nil {
		t.Fatal("NewHandler returned nil")
	}

	// Test that display can be initialized
	err = handler.display.Init()
	if err != nil {
		t.Fatalf("Failed to initialize display: %v", err)
	}
	defer handler.display.Close()

	// Test basic display operations
	err = handler.display.ClearScreen()
	if err != nil {
		t.Fatalf("Failed to clear display: %v", err)
	}

	lines := []string{"Test", "Line 1", "Line 2"}
	err = handler.display.PrintLines(0, lines)
	if err != nil {
		t.Fatalf("Failed to print lines: %v", err)
	}

	err = handler.display.Update()
	if err != nil {
		t.Fatalf("Failed to update display: %v", err)
	}
}

func TestConfigDefaults(t *testing.T) {
	cfg := NewConfig()
	if cfg == nil {
		t.Fatal("NewConfig returned nil")
	}
	if cfg.UpdateInterval != 5*time.Second {
		t.Errorf("Expected update interval 5s, got %v", cfg.UpdateInterval)
	}
	if cfg.ServerURL == "" {
		t.Error("Expected non-empty server URL")
	}
}

func TestShouldDisplayBeActive(t *testing.T) {
	// Create a fake display for testing
	d, err := display.NewDisplay().WithDriver(fakedriver.NewFakeSSD1306()).Build()
	if err != nil {
		t.Fatalf("Failed to build fake display: %v", err)
	}

	handler := NewHandler(d)

	tests := []struct {
		name              string
		displayTimeout    time.Duration
		mqttServerConfig  string
		timeSinceActivity time.Duration
		expected          bool
	}{
		{
			name:           "timeout disabled",
			displayTimeout: 0,
			expected:       true,
		},
		{
			name:              "no mqtt config, within timeout",
			displayTimeout:    5 * time.Minute,
			mqttServerConfig:  "",
			timeSinceActivity: 1 * time.Minute,
			expected:          true,
		},
		{
			name:              "no mqtt config, beyond timeout - should never blank",
			displayTimeout:    5 * time.Minute,
			mqttServerConfig:  "",
			timeSinceActivity: 10 * time.Minute,
			expected:          true, // Should never blank when no MQTT configured
		},
		{
			name:              "mqtt configured but client nil - should never blank",
			displayTimeout:    5 * time.Minute,
			mqttServerConfig:  "mqtt://localhost:1883",
			timeSinceActivity: 10 * time.Minute,
			expected:          true, // Should never blank when MQTT configured but unavailable
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up handler state
			handler.lastActivity = time.Now().Add(-tt.timeSinceActivity)
			// For the MQTT test case, ensure mqttClient is nil to simulate connection failure
			handler.mqttClient = nil

			result := handler.shouldDisplayBeActive(tt.displayTimeout, tt.mqttServerConfig)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}
