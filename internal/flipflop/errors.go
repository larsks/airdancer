package flipflop

import "errors"

var (
	ErrNoSwitches       = errors.New("at least one switch is required")
	ErrInvalidPeriod    = errors.New("period must be greater than 0")
	ErrInvalidDutyCycle = errors.New("duty cycle must be between 0 and 1")
	ErrAlreadyRunning   = errors.New("flipflop is already running")
	ErrNotRunning       = errors.New("flipflop is not running")
)