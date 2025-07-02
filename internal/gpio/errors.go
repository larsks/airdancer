package gpio

import "errors"

// Hardware initialization errors
var (
	ErrPeriphInitFailed = errors.New("failed to initialize periph.io")
	ErrPinNotFound      = errors.New("failed to find pin")
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
	ErrSwitchTurnOn  = errors.New("failed to turn on switch")
	ErrSwitchTurnOff = errors.New("failed to turn off switch")
)
