package blink

import "errors"

// Blink configuration errors
var (
	ErrSwitchRequired   = errors.New("switch is required")
	ErrInvalidFrequency = errors.New("frequency must be greater than 0")
)

// Blink operation errors
var (
	ErrAlreadyRunning = errors.New("blink is already running")
	ErrNotRunning     = errors.New("blink is not running")
)
