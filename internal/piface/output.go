package piface

import (
	"fmt"
	"log"
)

type PiFaceOutput struct {
	pf  *PiFace
	pin uint8
}

// PiFaceOutput methods
func (pfo *PiFaceOutput) setState(state bool) error {
	action := "off"
	value := uint8(0)
	if state {
		action = "on"
		value = 1
	}

	log.Printf("turn %s output %s", action, pfo)
	if err := pfo.pf.WriteOutput(pfo.pin, value); err != nil {
		if action == "on" {
			return fmt.Errorf("%w output %d: %v", ErrOutputTurnOn, pfo.pin, err)
		} else {
			return fmt.Errorf("%w output %d: %v", ErrOutputTurnOff, pfo.pin, err)
		}
	}
	return nil
}

func (pfo *PiFaceOutput) TurnOn() error {
	return pfo.setState(true)
}

func (pfo *PiFaceOutput) TurnOff() error {
	return pfo.setState(false)
}

func (pfo *PiFaceOutput) GetID() uint {
	return uint(pfo.pin)
}

func (pfo *PiFaceOutput) GetState() (bool, error) {
	val, err := pfo.pf.ReadOutput(pfo.pin)
	if err != nil {
		return false, fmt.Errorf("%w for output %d: %v", ErrOutputGetState, pfo.pin, err)
	}

	return val != 0, nil
}

func (pfo *PiFaceOutput) String() string {
	return fmt.Sprintf("%s:%d", pfo.pf, pfo.pin)
}
