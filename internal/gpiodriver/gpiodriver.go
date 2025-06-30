package gpiodriver

import (
	"fmt"
	"log"

	"github.com/larsks/airdancer/internal/switchdriver"
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/host/v3"
)

type (
	GpioSwitch struct {
		pin gpio.PinIO
	}

	GpioSwitchCollection struct {
		switches []switchdriver.Switch
	}
)

func NewGpioSwitchCollection(pins []string) (*GpioSwitchCollection, error) {
	if _, err := host.Init(); err != nil {
		return nil, fmt.Errorf("failed to init periph: %w", err)
	}

	switches := make([]switchdriver.Switch, len(pins))
	for i, pinName := range pins {
		pin := gpioreg.ByName(pinName)
		if pin == nil {
			return nil, fmt.Errorf("failed to find pin %s", pinName)
		}
		switches[i] = &GpioSwitch{
			pin: pin,
		}
	}

	return &GpioSwitchCollection{
		switches: switches,
	}, nil
}

func (sc *GpioSwitchCollection) Init() error {
	log.Printf("initializing gpio driver")
	for _, s := range sc.switches {
		if err := s.(*GpioSwitch).pin.Out(gpio.Low); err != nil {
			return fmt.Errorf("failed to set pin to output mode: %w", err)
		}
	}
	return nil
}

func (sc *GpioSwitchCollection) Close() error {
	log.Printf("closing gpio driver")
	for _, s := range sc.switches {
		if err := s.(*GpioSwitch).pin.Out(gpio.Low); err != nil {
			log.Printf("failed to reset pin to low: %s", err)
		}
	}
	return nil
}

func (sc *GpioSwitchCollection) CountSwitches() uint {
	return uint(len(sc.switches))
}

func (sc *GpioSwitchCollection) ListSwitches() []switchdriver.Switch {
	return sc.switches
}

func (sc *GpioSwitchCollection) GetSwitch(id uint) (switchdriver.Switch, error) {
	if id >= uint(len(sc.switches)) {
		return nil, fmt.Errorf("invalid switch id %d", id)
	}
	return sc.switches[id], nil
}

func (sc *GpioSwitchCollection) TurnOn() error {
	for _, s := range sc.switches {
		if err := s.TurnOn(); err != nil {
			return err
		}
	}
	return nil
}

func (sc *GpioSwitchCollection) TurnOff() error {
	for _, s := range sc.switches {
		if err := s.TurnOff(); err != nil {
			return err
		}
	}
	return nil
}

func (sc *GpioSwitchCollection) String() string {
	return fmt.Sprintf("gpio switch collection with %d switches", len(sc.switches))
}

func (s *GpioSwitch) TurnOn() error {
	log.Printf("activating switch %s", s)
	if err := s.pin.Out(gpio.High); err != nil {
		return fmt.Errorf("failed to turn on switch %s: %w", s, err)
	}
	return nil
}

func (s *GpioSwitch) TurnOff() error {
	log.Printf("deactivating switch %s", s)
	if err := s.pin.Out(gpio.Low); err != nil {
		return fmt.Errorf("failed to turn off switch %s: %w", s, err)
	}
	return nil
}

func (s *GpioSwitch) String() string {
	return s.pin.Name()
}
