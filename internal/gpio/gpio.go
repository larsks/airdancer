package gpio

import (
	"fmt"
	"log"

	"github.com/larsks/airdancer/internal/switchcollection"
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/host/v3"
)

type (
	GPIOSwitch struct {
		pin gpio.PinIO
	}

	GPIOSwitchCollection struct {
		offOnClose bool
		switches   []switchcollection.Switch
	}
)

func NewGPIOSwitchCollection(offOnClose bool, pins []string) (*GPIOSwitchCollection, error) {
	if _, err := host.Init(); err != nil {
		return nil, fmt.Errorf("failed to init periph: %w", err)
	}

	switches := make([]switchcollection.Switch, len(pins))
	for i, pinName := range pins {
		pin := gpioreg.ByName(pinName)
		if pin == nil {
			return nil, fmt.Errorf("failed to find pin %s", pinName)
		}
		switches[i] = &GPIOSwitch{
			pin: pin,
		}
	}

	return &GPIOSwitchCollection{
		offOnClose: offOnClose,
		switches:   switches,
	}, nil
}

func (sc *GPIOSwitchCollection) Init() error {
	log.Printf("initializing gpio driver")
	for _, s := range sc.switches {
		if err := s.(*GPIOSwitch).pin.Out(gpio.Low); err != nil {
			return fmt.Errorf("failed to set pin to output mode: %w", err)
		}
	}
	return nil
}

func (sc *GPIOSwitchCollection) Close() error {
	log.Printf("closing gpio driver")
	if sc.offOnClose {
		for _, s := range sc.switches {
			if err := s.(*GPIOSwitch).pin.Out(gpio.Low); err != nil {
				log.Printf("failed to reset pin to low: %s", err)
			}
		}
	}
	return nil
}

func (sc *GPIOSwitchCollection) CountSwitches() uint {
	return uint(len(sc.switches))
}

func (sc *GPIOSwitchCollection) ListSwitches() []switchcollection.Switch {
	return sc.switches
}

func (sc *GPIOSwitchCollection) GetSwitch(id uint) (switchcollection.Switch, error) {
	if id >= uint(len(sc.switches)) {
		return nil, fmt.Errorf("invalid switch id %d", id)
	}
	return sc.switches[id], nil
}

func (sc *GPIOSwitchCollection) TurnOn() error {
	for _, s := range sc.switches {
		if err := s.TurnOn(); err != nil {
			return err
		}
	}
	return nil
}

func (sc *GPIOSwitchCollection) TurnOff() error {
	for _, s := range sc.switches {
		if err := s.TurnOff(); err != nil {
			return err
		}
	}
	return nil
}

func (sc *GPIOSwitchCollection) GetState() (bool, error) {
	for _, s := range sc.switches {
		state, err := s.GetState()
		if err != nil {
			return false, err
		}
		if !state {
			return false, nil
		}
	}
	return true, nil
}

func (sc *GPIOSwitchCollection) GetDetailedState() ([]bool, error) {
	states := make([]bool, len(sc.switches))
	for i, s := range sc.switches {
		state, err := s.GetState()
		if err != nil {
			return nil, err
		}
		states[i] = state
	}
	return states, nil
}

func (sc *GPIOSwitchCollection) String() string {
	return fmt.Sprintf("gpio switch collection with %d switches", len(sc.switches))
}

func (s *GPIOSwitch) TurnOn() error {
	log.Printf("activating switch %s", s)
	if err := s.pin.Out(gpio.High); err != nil {
		return fmt.Errorf("failed to turn on switch %s: %w", s, err)
	}
	return nil
}

func (s *GPIOSwitch) TurnOff() error {
	log.Printf("deactivating switch %s", s)
	if err := s.pin.Out(gpio.Low); err != nil {
		return fmt.Errorf("failed to turn off switch %s: %w", s, err)
	}
	return nil
}

func (s *GPIOSwitch) GetState() (bool, error) {
	return s.pin.Read() == gpio.High, nil
}

func (s *GPIOSwitch) String() string {
	return s.pin.Name()
}
