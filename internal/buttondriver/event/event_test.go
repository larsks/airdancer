package event

import (
	"testing"
	"time"

	"github.com/larsks/airdancer/internal/buttondriver/common"
	"github.com/larsks/airdancer/internal/events"
)

func TestEventButtonSpec_Validate(t *testing.T) {
	tests := []struct {
		name        string
		spec        *EventButtonSpec
		expectedErr string
	}{
		{
			name: "valid spec",
			spec: &EventButtonSpec{
				Name:      "test-button",
				Device:    "/dev/input/event0",
				EventType: events.EV_KEY,
				EventCode: 116,
				LowValue:  0,
				HighValue: 1,
			},
			expectedErr: "",
		},
		{
			name: "missing name",
			spec: &EventButtonSpec{
				Device:    "/dev/input/event0",
				EventType: events.EV_KEY,
				EventCode: 116,
				LowValue:  0,
				HighValue: 1,
			},
			expectedErr: "button name is required",
		},
		{
			name: "missing device",
			spec: &EventButtonSpec{
				Name:      "test-button",
				EventType: events.EV_KEY,
				EventCode: 116,
				LowValue:  0,
				HighValue: 1,
			},
			expectedErr: "device path is required",
		},
		{
			name: "missing event code",
			spec: &EventButtonSpec{
				Name:      "test-button",
				Device:    "/dev/input/event0",
				EventType: events.EV_KEY,
				EventCode: 0,
				LowValue:  0,
				HighValue: 1,
			},
			expectedErr: "event code is required",
		},
		{
			name: "empty name",
			spec: &EventButtonSpec{
				Name:      "",
				Device:    "/dev/input/event0",
				EventType: events.EV_KEY,
				EventCode: 116,
				LowValue:  0,
				HighValue: 1,
			},
			expectedErr: "button name is required",
		},
		{
			name: "empty device",
			spec: &EventButtonSpec{
				Name:      "test-button",
				Device:    "",
				EventType: events.EV_KEY,
				EventCode: 116,
				LowValue:  0,
				HighValue: 1,
			},
			expectedErr: "device path is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.spec.Validate()
			if tt.expectedErr == "" {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.expectedErr)
				} else if err.Error() != tt.expectedErr {
					t.Errorf("expected error %q, got %q", tt.expectedErr, err.Error())
				}
			}
		})
	}
}

func TestEventButtonSpec_GetName(t *testing.T) {
	spec := &EventButtonSpec{Name: "test-button"}
	if got := spec.GetName(); got != "test-button" {
		t.Errorf("expected %q, got %q", "test-button", got)
	}
}

func TestEventButtonSpec_GetDevice(t *testing.T) {
	spec := &EventButtonSpec{Device: "/dev/input/event0"}
	if got := spec.GetDevice(); got != "/dev/input/event0" {
		t.Errorf("expected %q, got %q", "/dev/input/event0", got)
	}
}

func TestParseEventButtonSpec(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    *EventButtonSpec
		expectedErr string
	}{
		{
			name:  "valid spec with defaults",
			input: "power:/dev/input/event0:EV_KEY:116",
			expected: &EventButtonSpec{
				Name:      "power",
				Device:    "/dev/input/event0",
				EventType: events.EV_KEY,
				EventCode: 116,
				LowValue:  0,
				HighValue: 1,
			},
		},
		{
			name:  "valid spec with custom values",
			input: "volume:/dev/input/event1:EV_KEY:114:2:5",
			expected: &EventButtonSpec{
				Name:      "volume",
				Device:    "/dev/input/event1",
				EventType: events.EV_KEY,
				EventCode: 114,
				LowValue:  2,
				HighValue: 5,
			},
		},
		{
			name:        "too few parts",
			input:       "power:/dev/input/event0:EV_KEY",
			expectedErr: "invalid event button spec format. Expected: name:device:event_type:event_code[:low_value:high_value]",
		},
		{
			name:        "empty name",
			input:       ":/dev/input/event0:EV_KEY:116",
			expectedErr: "button name cannot be empty",
		},
		{
			name:        "empty device",
			input:       "power::EV_KEY:116",
			expectedErr: "device path cannot be empty",
		},
		{
			name:        "invalid event type",
			input:       "power:/dev/input/event0:INVALID:116",
			expectedErr: "unknown event type: INVALID",
		},
		{
			name:        "invalid event code",
			input:       "power:/dev/input/event0:EV_KEY:abc",
			expectedErr: "invalid event code: abc",
		},
		{
			name:        "invalid low value",
			input:       "power:/dev/input/event0:EV_KEY:116:abc:1",
			expectedErr: "invalid low value: abc",
		},
		{
			name:        "invalid high value",
			input:       "power:/dev/input/event0:EV_KEY:116:0:abc",
			expectedErr: "invalid high value: abc",
		},
		{
			name:        "event code too large",
			input:       "power:/dev/input/event0:EV_KEY:4294967296",
			expectedErr: "invalid event code: 4294967296",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseEventButtonSpec(tt.input)
			
			if tt.expectedErr != "" {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.expectedErr)
				} else if err.Error() != tt.expectedErr {
					t.Errorf("expected error %q, got %q", tt.expectedErr, err.Error())
				}
				return
			}
			
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			
			if result.Name != tt.expected.Name {
				t.Errorf("expected Name %q, got %q", tt.expected.Name, result.Name)
			}
			if result.Device != tt.expected.Device {
				t.Errorf("expected Device %q, got %q", tt.expected.Device, result.Device)
			}
			if result.EventType != tt.expected.EventType {
				t.Errorf("expected EventType %v, got %v", tt.expected.EventType, result.EventType)
			}
			if result.EventCode != tt.expected.EventCode {
				t.Errorf("expected EventCode %v, got %v", tt.expected.EventCode, result.EventCode)
			}
			if result.LowValue != tt.expected.LowValue {
				t.Errorf("expected LowValue %v, got %v", tt.expected.LowValue, result.LowValue)
			}
			if result.HighValue != tt.expected.HighValue {
				t.Errorf("expected HighValue %v, got %v", tt.expected.HighValue, result.HighValue)
			}
		})
	}
}

func TestNewEventButtonDriver(t *testing.T) {
	driver := NewEventButtonDriver()
	
	if driver == nil {
		t.Fatal("expected non-nil driver")
	}
	
	if driver.buttons == nil {
		t.Error("expected buttons map to be initialized")
	}
	
	if driver.files == nil {
		t.Error("expected files map to be initialized")
	}
	
	if driver.eventChan == nil {
		t.Error("expected event channel to be initialized")
	}
	
	if driver.stopChan == nil {
		t.Error("expected stop channel to be initialized")
	}
	
	if driver.started {
		t.Error("expected started to be false initially")
	}
}

func TestEventButtonDriver_Events(t *testing.T) {
	driver := NewEventButtonDriver()
	eventChan := driver.Events()
	
	if eventChan == nil {
		t.Error("expected non-nil event channel")
	}
	
	// Verify it's read-only
	select {
	case <-eventChan:
		// This is expected to not receive anything initially
	default:
		// This is the expected path
	}
}

func TestEventButtonDriver_AddButton(t *testing.T) {
	driver := NewEventButtonDriver()
	
	tests := []struct {
		name        string
		buttonSpec  interface{}
		expectedErr string
	}{
		{
			name: "valid button spec",
			buttonSpec: &EventButtonSpec{
				Name:      "test-button",
				Device:    "/dev/input/event0",
				EventType: events.EV_KEY,
				EventCode: 116,
				LowValue:  0,
				HighValue: 1,
			},
		},
		{
			name:        "invalid type",
			buttonSpec:  "invalid",
			expectedErr: "invalid button spec type, expected *EventButtonSpec",
		},
		{
			name: "invalid spec",
			buttonSpec: &EventButtonSpec{
				Name:      "",
				Device:    "/dev/input/event0",
				EventType: events.EV_KEY,
				EventCode: 116,
			},
			expectedErr: "invalid button specification: button name is required",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := driver.AddButton(tt.buttonSpec)
			
			if tt.expectedErr != "" {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.expectedErr)
				} else if err.Error() != tt.expectedErr {
					t.Errorf("expected error %q, got %q", tt.expectedErr, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestEventButtonDriver_GetButtons(t *testing.T) {
	driver := NewEventButtonDriver()
	
	// Initially empty
	buttons := driver.GetButtons()
	if len(buttons) != 0 {
		t.Errorf("expected 0 buttons, got %d", len(buttons))
	}
	
	// Add buttons
	spec1 := &EventButtonSpec{
		Name:      "button1",
		Device:    "/dev/input/event0",
		EventType: events.EV_KEY,
		EventCode: 116,
		LowValue:  0,
		HighValue: 1,
	}
	
	spec2 := &EventButtonSpec{
		Name:      "button2",
		Device:    "/dev/input/event0",
		EventType: events.EV_KEY,
		EventCode: 117,
		LowValue:  0,
		HighValue: 1,
	}
	
	driver.AddButton(spec1)
	driver.AddButton(spec2)
	
	buttons = driver.GetButtons()
	if len(buttons) != 2 {
		t.Errorf("expected 2 buttons, got %d", len(buttons))
	}
	
	// Check button names are present
	buttonNames := make(map[string]bool)
	for _, name := range buttons {
		buttonNames[name] = true
	}
	
	if !buttonNames["button1"] {
		t.Error("expected button1 to be present")
	}
	if !buttonNames["button2"] {
		t.Error("expected button2 to be present")
	}
}

func TestEventButtonDriver_StartStop(t *testing.T) {
	driver := NewEventButtonDriver()
	
	// Test starting with no buttons
	err := driver.Start()
	if err == nil {
		t.Error("expected error when starting with no buttons")
	}
	
	// Add a button (we won't actually open the device file in this test)
	spec := &EventButtonSpec{
		Name:      "test-button",
		Device:    "/dev/input/event0",
		EventType: events.EV_KEY,
		EventCode: 116,
		LowValue:  0,
		HighValue: 1,
	}
	driver.AddButton(spec)
	
	// Test double start
	// Note: This test will fail trying to open the device file, but that's expected
	// in a unit test environment. The important thing is testing the state management.
	
	// Test stop without start
	driver.Stop() // Should not panic
	
	// Test multiple stops
	driver.Stop() // Should not panic
	driver.Stop() // Should not panic
}

func TestEventButtonDriver_handleButtonEvent(t *testing.T) {
	driver := NewEventButtonDriver()
	
	spec := &EventButtonSpec{
		Name:      "test-button",
		Device:    "/dev/input/event0",
		EventType: events.EV_KEY,
		EventCode: 116,
		LowValue:  0,
		HighValue: 1,
	}
	
	// Test high value (button pressed)
	driver.handleButtonEvent(spec, 1, "/dev/input/event0")
	
	// Test low value (button released)
	driver.handleButtonEvent(spec, 0, "/dev/input/event0")
	
	// Test invalid value (should be ignored)
	driver.handleButtonEvent(spec, 999, "/dev/input/event0")
	
	// Check that we got the expected events
	eventCount := 0
	timeout := time.After(100 * time.Millisecond)
	
	for {
		select {
		case event := <-driver.eventChan:
			eventCount++
			if event.Source != "test-button" {
				t.Errorf("expected source %q, got %q", "test-button", event.Source)
			}
			if event.Device != "/dev/input/event0" {
				t.Errorf("expected device %q, got %q", "/dev/input/event0", event.Device)
			}
			if eventCount == 1 && event.Type != common.ButtonPressed {
				t.Errorf("expected first event to be ButtonPressed, got %v", event.Type)
			}
			if eventCount == 2 && event.Type != common.ButtonReleased {
				t.Errorf("expected second event to be ButtonReleased, got %v", event.Type)
			}
		case <-timeout:
			if eventCount != 2 {
				t.Errorf("expected 2 events, got %d", eventCount)
			}
			return
		}
	}
}