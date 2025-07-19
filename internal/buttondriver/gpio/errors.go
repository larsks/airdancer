package gpio

import "errors"

var (
	ErrPinNotFound      = errors.New("GPIO pin not found")
	ErrPinConfig        = errors.New("failed to configure GPIO pin")
	ErrGPIOChipOpen     = errors.New("failed to open GPIO chip")
	ErrNoPinsConfigured = errors.New("no GPIO pins configured")
)
