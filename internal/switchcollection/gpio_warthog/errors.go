package gpio_warthog

import "errors"

// Hardware initialization errors
var (
	ErrGPIOChipOpenFailed = errors.New("failed to open GPIO chip")
	ErrLineNotFound       = errors.New("failed to find GPIO line")
	ErrLineRequestFailed  = errors.New("failed to request GPIO line")
)

// Pin configuration errors
var (
	ErrPinOutputMode = errors.New("failed to set pin to output mode")
)

// Switch collection errors
var (
	ErrInvalidSwitchID = errors.New("invalid switch id")
)

// Switch operation errors
var (
	ErrSwitchTurnOn   = errors.New("failed to turn on switch")
	ErrSwitchTurnOff  = errors.New("failed to turn off switch")
	ErrSwitchGetState = errors.New("failed to get switch state")
)
