package gpio_warthog

import (
	"fmt"
	"log"

	"github.com/larsks/airdancer/internal/gpio"
	"github.com/larsks/airdancer/internal/switchcollection"
	"github.com/warthog618/go-gpiocdev"
)

type (
	WarthogGPIOSwitch struct {
		line     *gpiocdev.Line
		polarity gpio.Polarity
		lineNum  int
		state    bool // Track the logical state
	}

	WarthogGPIOSwitchCollection struct {
		chip       *gpiocdev.Chip
		offOnClose bool
		switches   []switchcollection.Switch
	}
)

type PinConfig struct {
	LineNum  int
	Polarity gpio.Polarity
}

func ParsePinConfig(pinSpec string) (PinConfig, error) {
	// Use common GPIO package for parsing
	parsedPin, err := gpio.ParsePin(pinSpec)
	if err != nil {
		return PinConfig{}, fmt.Errorf("invalid GPIO pin specification: %w", err)
	}

	return PinConfig{
		LineNum:  parsedPin.LineNum,
		Polarity: parsedPin.Polarity,
	}, nil
}

func NewGPIOSwitchCollection(offOnClose bool, pins []string) (*WarthogGPIOSwitchCollection, error) {
	// Open GPIO chip (typically /dev/gpiochip0 on Raspberry Pi)
	chip, err := gpiocdev.NewChip("gpiochip0")
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGPIOChipOpenFailed, err)
	}

	switches := make([]switchcollection.Switch, len(pins))
	for i, pinSpec := range pins {
		pinConfig, err := ParsePinConfig(pinSpec)
		if err != nil {
			chip.Close() //nolint:errcheck
			return nil, err
		}

		// Request the GPIO line for output
		line, err := chip.RequestLine(pinConfig.LineNum, gpiocdev.AsOutput(0))
		if err != nil {
			chip.Close() //nolint:errcheck
			return nil, fmt.Errorf("%w: line %d: %v", ErrLineRequestFailed, pinConfig.LineNum, err)
		}

		switches[i] = &WarthogGPIOSwitch{
			line:     line,
			polarity: pinConfig.Polarity,
			lineNum:  pinConfig.LineNum,
			state:    false, // Start in off state
		}
	}

	return &WarthogGPIOSwitchCollection{
		chip:       chip,
		offOnClose: offOnClose,
		switches:   switches,
	}, nil
}

func (sc *WarthogGPIOSwitchCollection) Init() error {
	log.Printf("initializing warthog gpio driver")
	for _, s := range sc.switches {
		gpioSwitch := s.(*WarthogGPIOSwitch)
		if err := gpioSwitch.TurnOff(); err != nil {
			return fmt.Errorf("%w: %v", ErrPinOutputMode, err)
		}
	}
	return nil
}

func (sc *WarthogGPIOSwitchCollection) Close() error {
	log.Printf("closing warthog gpio driver")
	if sc.offOnClose {
		for _, s := range sc.switches {
			gpioSwitch := s.(*WarthogGPIOSwitch)
			if err := gpioSwitch.TurnOff(); err != nil {
				log.Printf("failed to reset pin to off state: %s", err)
			}
		}
	}

	// Close all GPIO lines
	for _, s := range sc.switches {
		gpioSwitch := s.(*WarthogGPIOSwitch)
		if err := gpioSwitch.line.Close(); err != nil {
			log.Printf("failed to close GPIO line %d: %s", gpioSwitch.lineNum, err)
		}
	}

	// Close the chip
	if err := sc.chip.Close(); err != nil {
		log.Printf("failed to close GPIO chip: %s", err)
	}

	return nil
}

func (sc *WarthogGPIOSwitchCollection) CountSwitches() uint {
	return uint(len(sc.switches))
}

func (sc *WarthogGPIOSwitchCollection) ListSwitches() []switchcollection.Switch {
	return sc.switches
}

func (sc *WarthogGPIOSwitchCollection) GetSwitch(id uint) (switchcollection.Switch, error) {
	if id >= uint(len(sc.switches)) {
		return nil, fmt.Errorf("%w: %d", ErrInvalidSwitchID, id)
	}
	return sc.switches[id], nil
}

func (sc *WarthogGPIOSwitchCollection) TurnOn() error {
	for _, s := range sc.switches {
		if err := s.TurnOn(); err != nil {
			return err
		}
	}
	return nil
}

func (sc *WarthogGPIOSwitchCollection) TurnOff() error {
	for _, s := range sc.switches {
		if err := s.TurnOff(); err != nil {
			return err
		}
	}
	return nil
}

func (sc *WarthogGPIOSwitchCollection) GetState() (bool, error) {
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

func (sc *WarthogGPIOSwitchCollection) GetDetailedState() ([]bool, error) {
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

// IsDisabled returns false since Warthog GPIO switch collections are never disabled
func (sc *WarthogGPIOSwitchCollection) IsDisabled() bool {
	return false
}

func (sc *WarthogGPIOSwitchCollection) String() string {
	return fmt.Sprintf("warthog gpio switch collection with %d switches", len(sc.switches))
}

func (s *WarthogGPIOSwitch) TurnOn() error {
	log.Printf("activating switch %s", s)
	onLevel := s.getOnLevel()
	if err := s.line.SetValue(onLevel); err != nil {
		return fmt.Errorf("%w %s: %v", ErrSwitchTurnOn, s, err)
	}
	s.state = true
	return nil
}

func (s *WarthogGPIOSwitch) TurnOff() error {
	log.Printf("deactivating switch %s", s)
	offLevel := s.getOffLevel()
	if err := s.line.SetValue(offLevel); err != nil {
		return fmt.Errorf("%w %s: %v", ErrSwitchTurnOff, s, err)
	}
	s.state = false
	return nil
}

func (s *WarthogGPIOSwitch) GetState() (bool, error) {
	// Read actual pin state from hardware using go-gpiocdev
	pinLevel, err := s.line.Value()
	if err != nil {
		return false, fmt.Errorf("%w %s: %v", ErrSwitchGetState, s, err)
	}

	onLevel := s.getOnLevel()
	isOn := pinLevel == onLevel
	return isOn, nil
}

// IsDisabled returns false since Warthog GPIO switches are never disabled
func (s *WarthogGPIOSwitch) IsDisabled() bool {
	return false
}

func (s *WarthogGPIOSwitch) String() string {
	return fmt.Sprintf("GPIO%d", s.lineNum)
}

func (s *WarthogGPIOSwitch) getOnLevel() int {
	if s.polarity == gpio.ActiveHigh {
		return 1
	}
	return 0
}

func (s *WarthogGPIOSwitch) getOffLevel() int {
	if s.polarity == gpio.ActiveHigh {
		return 0
	}
	return 1
}
