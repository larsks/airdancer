package gpio

import "errors"

var (
	ErrPinNotFound      = errors.New("GPIO pin not found")
	ErrPinConfig        = errors.New("failed to configure GPIO pin")
	ErrPeriphInit       = errors.New("failed to initialize periph.io")
	ErrNoPinsConfigured = errors.New("no GPIO pins configured")
)
