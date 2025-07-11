package gpio

import (
	"fmt"
	"strings"
	"time"

	"github.com/larsks/airdancer/internal/buttondriver/common"
)

// GPIOButtonSpec represents a GPIO button specification
type GPIOButtonSpec struct {
	// Name is the button identifier
	Name string

	// Pin is the GPIO pin name (e.g., "GPIO16", "GPIO18")
	Pin string

	// ActiveHigh indicates if the button is active-high (true) or active-low (false)
	ActiveHigh bool

	// PullMode specifies the pull resistor configuration
	PullMode PullMode

	// DebounceDelay is the debounce delay for this specific button
	DebounceDelay *time.Duration // nil means use driver default
}

// NewGPIOButtonSpec creates a new GPIO button specification
func NewGPIOButtonSpec(name, pin string) *GPIOButtonSpec {
	return &GPIOButtonSpec{
		Name:     name,
		Pin:      pin,
		PullMode: PullAuto,
	}
}

func (b *GPIOButtonSpec) WithPullMode(pullMode PullMode) *GPIOButtonSpec {
	b.PullMode = pullMode
	return b
}

func (b *GPIOButtonSpec) WithActiveHigh() *GPIOButtonSpec {
	b.ActiveHigh = true
	return b
}

func (b *GPIOButtonSpec) WithDebounceDelay(delay time.Duration) *GPIOButtonSpec {
	b.DebounceDelay = &delay
	return b
}

// ParseGPIOButtonSpec parses a GPIO button specification from a string
// Format: "name:pin[:active-high|active-low][:pull-none|pull-up|pull-down|pull-auto]"
// Examples: "button1:GPIO16", "button2:GPIO18:active-low:pull-up"
func ParseGPIOButtonSpec(spec string) (*GPIOButtonSpec, error) {
	parts := strings.Split(spec, ":")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid GPIO button spec format: %s (expected name:pin[:options])", spec)
	}

	name := strings.TrimSpace(parts[0])
	pin := strings.TrimSpace(parts[1])

	if name == "" {
		return nil, fmt.Errorf("button name cannot be empty")
	}
	if pin == "" {
		return nil, fmt.Errorf("GPIO pin cannot be empty")
	}

	// Default values
	activeHigh := true
	pullMode := PullAuto

	// Parse optional parameters
	for i := 2; i < len(parts); i++ {
		param := strings.ToLower(strings.TrimSpace(parts[i]))
		switch param {
		case "active-high":
			activeHigh = true
		case "active-low":
			activeHigh = false
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

	return &GPIOButtonSpec{
		Name:       name,
		Pin:        pin,
		ActiveHigh: activeHigh,
		PullMode:   pullMode,
	}, nil
}

// GetName returns the button's name/identifier
func (spec *GPIOButtonSpec) GetName() string {
	return spec.Name
}

// GetDevice returns the device path/name
func (spec *GPIOButtonSpec) GetDevice() string {
	return spec.Pin
}

// Validate checks if the button specification is valid
func (spec *GPIOButtonSpec) Validate() error {
	if spec.Name == "" {
		return fmt.Errorf("button name cannot be empty")
	}
	if spec.Pin == "" {
		return fmt.Errorf("GPIO pin cannot be empty")
	}
	return nil
}

// String returns a string representation of the GPIO button spec
func (spec *GPIOButtonSpec) String() string {
	activeStr := "active-high"
	if !spec.ActiveHigh {
		activeStr = "active-low"
	}

	var pullStr string
	switch spec.PullMode {
	case PullNone:
		pullStr = "pull-none"
	case PullUp:
		pullStr = "pull-up"
	case PullDown:
		pullStr = "pull-down"
	case PullAuto:
		pullStr = "pull-auto"
	}

	return fmt.Sprintf("%s:%s:%s:%s", spec.Name, spec.Pin, activeStr, pullStr)
}

// Ensure GPIOButtonSpec implements the common.ButtonSpec interface
var _ common.ButtonSpec = (*GPIOButtonSpec)(nil)

