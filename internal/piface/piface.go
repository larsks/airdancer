package piface

import (
	"fmt"
	"log"

	"github.com/larsks/airdancer/internal/switchcollection"
)

const (
	NUMBER_OF_OUTPUTS = 8
)

func (pf *PiFace) ReadInputs() (uint8, error) {
	val, err := pf.readRegister(GPIOB)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrReadInputs, err)
	}

	return val ^ 0xFF, nil
}

func (pf *PiFace) ReadInput(pin uint8) (uint8, error) {
	if err := validatePin(pin); err != nil {
		return 0, fmt.Errorf("%w: %v", ErrReadInput, err)
	}

	vec, err := pf.ReadInputs()
	if err != nil {
		return 0, fmt.Errorf("%w pin %d: %v", ErrReadInput, pin, err)
	}

	return (vec >> pin) & 0x1, nil
}

func (pf *PiFace) WriteOutputs(val uint8) error {
	if err := pf.writeRegister(GPIOA, val); err != nil {
		return fmt.Errorf("%w: %v", ErrWriteOutputs, err)
	}
	return nil
}

func (pf *PiFace) WriteOutput(pin uint8, val uint8) error {
	if err := validatePin(pin); err != nil {
		return fmt.Errorf("%w: %v", ErrWriteOutput, err)
	}
	if val > 1 {
		return fmt.Errorf("%w: %d (must be 0 or 1)", ErrInvalidOutputValue, val)
	}

	outputs, err := pf.ReadOutputs()
	if err != nil {
		return fmt.Errorf("%w pin %d: %v", ErrWriteOutput, pin, err)
	}

	newOutputs := setBit(outputs, pin, val == 1)
	if err := pf.WriteOutputs(newOutputs); err != nil {
		return fmt.Errorf("%w pin %d: %v", ErrWriteOutput, pin, err)
	}

	return nil
}

func (pf *PiFace) ReadOutputs() (uint8, error) {
	val, err := pf.readRegister(GPIOA)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrReadOutputs, err)
	}
	return val, nil
}

func (pf *PiFace) ReadOutput(pin uint8) (uint8, error) {
	if err := validatePin(pin); err != nil {
		return 0, fmt.Errorf("%w: %v", ErrReadOutput, err)
	}

	val, err := pf.ReadOutputs()
	if err != nil {
		return 0, fmt.Errorf("%w pin %d: %v", ErrReadOutput, pin, err)
	}

	return (val >> pin) & 1, nil
}

// String implements the Stringer interface for PiFace
func (pf *PiFace) String() string {
	return fmt.Sprintf("piface:%s", pf.spiPortName)
}

// SwitchCollection interface implementation
func (pf *PiFace) CountSwitches() uint {
	return NUMBER_OF_OUTPUTS
}

func (pf *PiFace) ListSwitches() []switchcollection.Switch {
	var switches []switchcollection.Switch
	for i := range NUMBER_OF_OUTPUTS {
		if sw, err := pf.GetSwitch(uint(i)); err == nil {
			switches = append(switches, sw)
		}
	}

	return switches
}

func (pf *PiFace) GetSwitch(id uint) (switchcollection.Switch, error) {
	if id > 7 {
		return nil, fmt.Errorf("%w: %d (must be 0-7)", ErrInvalidSwitchID, id)
	}
	return &PiFaceOutput{
		pf:  pf,
		pin: uint8(id),
	}, nil
}

func (pf *PiFace) TurnOn() error {
	log.Printf("turn on all switches on %s", pf)
	if err := pf.WriteOutputs(0xff); err != nil {
		return fmt.Errorf("%w: %v", ErrSwitchTurnOn, err)
	}
	return nil
}

func (pf *PiFace) TurnOff() error {
	log.Printf("turn off all switches on %s", pf)
	if err := pf.WriteOutputs(0x0); err != nil {
		return fmt.Errorf("%w: %v", ErrSwitchTurnOff, err)
	}
	return nil
}

func (pf *PiFace) GetState() (bool, error) {
	outputs, err := pf.ReadOutputs()
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrGetState, err)
	}
	return outputs == 0xFF, nil
}

func (pf *PiFace) GetDetailedState() ([]bool, error) {
	outputs, err := pf.ReadOutputs()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGetDetailedState, err)
	}

	states := make([]bool, NUMBER_OF_OUTPUTS)
	for i := 0; i < NUMBER_OF_OUTPUTS; i++ {
		states[i] = getBit(outputs, uint8(i))
	}
	return states, nil
}
