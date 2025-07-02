package api

import "errors"

// Driver initialization errors
var (
	ErrPiFaceInitFailed = errors.New("failed to open PiFace")
	ErrGPIOInitFailed   = errors.New("failed to create gpio driver")
	ErrUnknownDriver    = errors.New("unknown driver")
	ErrDriverInitFailed = errors.New("failed to initialize driver")
)

// Switch initialization errors
var (
	ErrSwitchInitFailed = errors.New("failed to initialize switches")
)

// Server operation errors
var (
	ErrServerShutdownFailed = errors.New("server shutdown failed")
)
