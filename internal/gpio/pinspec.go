package gpio

import (
	"fmt"
	"strconv"
	"strings"
)

// Polarity represents the electrical polarity of a GPIO pin
type Polarity int

const (
	ActiveHigh Polarity = iota
	ActiveLow
)

// PullMode represents the pull resistor configuration
type PullMode int

const (
	PullNone PullMode = iota
	PullUp
	PullDown
	PullAuto // Automatically choose based on polarity
)

// PinSpec represents a parsed GPIO pin specification
type PinSpec struct {
	// LineNum is the GPIO line number (e.g., 18 for GPIO18)
	LineNum int
	
	// Polarity indicates if the pin is active-high or active-low
	Polarity Polarity
	
	// PullMode specifies the pull resistor configuration
	PullMode PullMode
}

// ParsePin parses a GPIO pin specification string
// Format: "pin[:active-high|active-low][:pull-none|pull-up|pull-down|pull-auto]"
// Examples: "GPIO18", "GPIO18:active-low", "18:active-low:pull-up"
func ParsePin(pinSpec string) (*PinSpec, error) {
	parts := strings.Split(pinSpec, ":")
	if len(parts) < 1 {
		return nil, fmt.Errorf("invalid pin specification: %s", pinSpec)
	}

	// Parse GPIO pin number (supports both "GPIO18" and "18" formats)
	lineNum, err := ParsePinNumber(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid GPIO pin: %s", parts[0])
	}

	// Default values
	polarity := ActiveHigh
	pullMode := PullAuto

	// Parse optional parameters
	for i := 1; i < len(parts); i++ {
		param := strings.ToLower(strings.TrimSpace(parts[i]))
		switch param {
		case "active-high":
			polarity = ActiveHigh
		case "active-low":
			polarity = ActiveLow
		case "pull-none":
			pullMode = PullNone
		case "pull-up":
			pullMode = PullUp
		case "pull-down":
			pullMode = PullDown
		case "pull-auto":
			pullMode = PullAuto
		default:
			return nil, fmt.Errorf("unknown parameter: %s", param)
		}
	}

	return &PinSpec{
		LineNum:  lineNum,
		Polarity: polarity,
		PullMode: pullMode,
	}, nil
}

// ParsePinNumber parses a GPIO pin name (e.g., "GPIO16") and returns the line number
// Supports both "GPIO<number>" and "<number>" formats
func ParsePinNumber(pinName string) (int, error) {
	// Handle direct number format (e.g., "16")
	if lineNum, err := strconv.Atoi(pinName); err == nil {
		return lineNum, nil
	}

	// Handle GPIO prefix format (e.g., "GPIO16")
	if strings.HasPrefix(strings.ToUpper(pinName), "GPIO") {
		numStr := strings.TrimPrefix(strings.ToUpper(pinName), "GPIO")
		if lineNum, err := strconv.Atoi(numStr); err == nil {
			return lineNum, nil
		}
	}

	return 0, fmt.Errorf("invalid GPIO pin format: %s (expected format: GPIO<number> or <number>)", pinName)
}

// String returns a string representation of the polarity
func (p Polarity) String() string {
	switch p {
	case ActiveHigh:
		return "active-high"
	case ActiveLow:
		return "active-low"
	default:
		return "unknown"
	}
}

// String returns a string representation of the pull mode
func (pm PullMode) String() string {
	switch pm {
	case PullNone:
		return "pull-none"
	case PullUp:
		return "pull-up"
	case PullDown:
		return "pull-down"
	case PullAuto:
		return "pull-auto"
	default:
		return "unknown"
	}
}

// String returns a string representation of the pin specification
func (ps *PinSpec) String() string {
	return fmt.Sprintf("GPIO%d:%s:%s", ps.LineNum, ps.Polarity, ps.PullMode)
}