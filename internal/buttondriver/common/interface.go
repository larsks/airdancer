package common

import "time"

// ButtonEventType represents the type of button event
type ButtonEventType int

const (
	ButtonPressed ButtonEventType = iota
	ButtonReleased
)

// ButtonEvent represents a button press or release event
type ButtonEvent struct {
	// Source identifies which button generated the event
	Source string

	// Type indicates if this is a press or release event
	Type ButtonEventType

	// Timestamp when the event occurred
	Timestamp time.Time

	// Device is the device path/name (e.g., "/dev/input/event0" or "GPIO16")
	Device string

	// Metadata can store additional implementation-specific data
	Metadata map[string]interface{}
}

// ButtonDriver is the common interface for all button implementations
type ButtonDriver interface {
	// Events returns a channel that delivers button events
	Events() <-chan ButtonEvent

	// Start begins monitoring for button events
	Start() error

	// Stop stops monitoring and closes the events channel
	Stop()

	// AddButton adds a button to be monitored
	// The buttonSpec parameter format is implementation-specific
	AddButton(buttonSpec interface{}) error

	// GetButtons returns a list of button sources being monitored
	GetButtons() []string
}

// ButtonSpec is a common interface for button specifications
type ButtonSpec interface {
	// GetName returns the button's name/identifier
	GetName() string

	// GetDevice returns the device path/name
	GetDevice() string

	// Validate checks if the button specification is valid
	Validate() error
}

// String returns a human-readable representation of the button event type
func (bet ButtonEventType) String() string {
	switch bet {
	case ButtonPressed:
		return "PRESSED"
	case ButtonReleased:
		return "RELEASED"
	default:
		return "UNKNOWN"
	}
}

// IsPressed returns true if this is a button press event
func (be ButtonEvent) IsPressed() bool {
	return be.Type == ButtonPressed
}

// IsReleased returns true if this is a button release event
func (be ButtonEvent) IsReleased() bool {
	return be.Type == ButtonReleased
}
