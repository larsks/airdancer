package piface

import "errors"

// Pin and validation errors
var (
	ErrInvalidPin         = errors.New("invalid pin number")
	ErrInvalidOutputValue = errors.New("invalid output value")
)

// Hardware initialization and connection errors
var (
	ErrPeriphInitFailed = errors.New("failed to initialize periph.io")
	ErrSPIPortOpen      = errors.New("failed to open SPI port")
	ErrSPIConnect       = errors.New("failed to connect to SPI")
	ErrTooManySwitches  = errors.New("cannot more switches than available outputs")
)

// Register operation errors
var (
	ErrRegisterWrite = errors.New("failed to write register")
	ErrRegisterRead  = errors.New("failed to read register")
)

// GPIO operation errors
var (
	ErrReadInputs   = errors.New("failed to read inputs")
	ErrReadInput    = errors.New("failed to read input")
	ErrWriteOutputs = errors.New("failed to write outputs")
	ErrWriteOutput  = errors.New("failed to write output")
	ErrReadOutputs  = errors.New("failed to read outputs")
	ErrReadOutput   = errors.New("failed to read output")
)

// Switch collection errors
var (
	ErrInvalidSwitchID  = errors.New("invalid switch id")
	ErrSwitchTurnOn     = errors.New("failed to turn on switches")
	ErrSwitchTurnOff    = errors.New("failed to turn off switches")
	ErrGetState         = errors.New("failed to get state")
	ErrGetDetailedState = errors.New("failed to get detailed state")
)

// Output operation errors
var (
	ErrOutputTurnOn   = errors.New("failed to turn on output")
	ErrOutputTurnOff  = errors.New("failed to turn off output")
	ErrOutputGetState = errors.New("failed to get output state")
)
