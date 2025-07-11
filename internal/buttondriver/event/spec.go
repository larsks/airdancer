package event

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/larsks/airdancer/internal/events"
)

// EventButtonSpec represents a button specification for input event devices
type EventButtonSpec struct {
	Name      string
	Device    string
	EventType events.EventType
	EventCode uint32
	LowValue  uint32
	HighValue uint32
}

// GetName returns the button's name
func (spec *EventButtonSpec) GetName() string {
	return spec.Name
}

// GetDevice returns the device path
func (spec *EventButtonSpec) GetDevice() string {
	return spec.Device
}

// Validate checks if the button specification is valid
func (spec *EventButtonSpec) Validate() error {
	if spec.Name == "" {
		return fmt.Errorf("button name is required")
	}
	if spec.Device == "" {
		return fmt.Errorf("device path is required")
	}
	if spec.EventCode == 0 {
		return fmt.Errorf("event code is required")
	}
	return nil
}

// ParseEventButtonSpec parses a button specification string
// Format: name:device:event_type:event_code[:low_value:high_value]
// Example: "power:/dev/input/event0:EV_KEY:116" or "power:/dev/input/event0:EV_KEY:116:0:1"
func ParseEventButtonSpec(spec string) (*EventButtonSpec, error) {
	parts := strings.Split(spec, ":")
	if len(parts) < 4 {
		return nil, fmt.Errorf("invalid event button spec format. Expected: name:device:event_type:event_code[:low_value:high_value]")
	}

	name := parts[0]
	device := parts[1]
	eventTypeStr := parts[2]
	eventCodeStr := parts[3]

	if name == "" {
		return nil, fmt.Errorf("button name cannot be empty")
	}
	if device == "" {
		return nil, fmt.Errorf("device path cannot be empty")
	}

	// Parse event type
	eventType, ok := events.GetEventTypeName(eventTypeStr)
	if !ok {
		return nil, fmt.Errorf("unknown event type: %s", eventTypeStr)
	}

	// Parse event code
	eventCode, err := strconv.ParseUint(eventCodeStr, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid event code: %s", eventCodeStr)
	}

	// Default values
	lowValue := uint32(0)
	highValue := uint32(1)

	// Parse optional low/high values
	if len(parts) >= 6 {
		if lowVal, err := strconv.ParseUint(parts[4], 10, 32); err == nil {
			lowValue = uint32(lowVal)
		} else {
			return nil, fmt.Errorf("invalid low value: %s", parts[4])
		}

		if highVal, err := strconv.ParseUint(parts[5], 10, 32); err == nil {
			highValue = uint32(highVal)
		} else {
			return nil, fmt.Errorf("invalid high value: %s", parts[5])
		}
	}

	return &EventButtonSpec{
		Name:      name,
		Device:    device,
		EventType: eventType,
		EventCode: uint32(eventCode),
		LowValue:  lowValue,
		HighValue: highValue,
	}, nil
}