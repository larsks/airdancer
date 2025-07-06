package blink

import "errors"

// Blink configuration errors
var (
	ErrSwitchRequired   = errors.New("switch is required")
	ErrInvalidPeriod    = errors.New("period must be greater than 0")
	ErrInvalidDutyCycle = errors.New("period must be between 0 and 1")
)

// Blink operation errors
var (
	ErrAlreadyRunning = errors.New("blink is already running")
	ErrNotRunning     = errors.New("blink is not running")
)
