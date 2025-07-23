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
	return pf.maxSwitches
}

func (pf *PiFace) ListSwitches() []switchcollection.Switch {
	var switches []switchcollection.Switch
	for i := range pf.maxSwitches {
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

func setLowerBits(original, newValues uint8, writableBits uint8) uint8 {
	// 1. Create a mask for the bits we want to update.
	//    If writableBits is N, we need a mask with N ones at the end.
	//    (1 << N) creates a 1 at position N. Subtracting 1 flips all lower bits to 1.
	//    Example (writableBits = 6):
	//    1 << 6  -> 01000000
	//    - 1     -> 00111111 (this is our updateMask, 0x3F)
	updateMask := uint8((1 << writableBits) - 1)

	// 2. Use the bit clear operator (&^) to clear the lower bits in the original value.
	//    This is equivalent to `original & (^updateMask)`.
	//    Example (original = 11010101, updateMask = 00111111):
	//    11010101 &^ 00111111  ->  11000000
	preservedBits := original &^ updateMask

	// 3. Ensure the new values only affect the bits within the mask.
	//    Example (newValues = 00001100, updateMask = 00111111):
	//    00001100 & 00111111  ->  00001100
	newBits := newValues & updateMask

	// 4. Combine the preserved upper bits with the new lower bits.
	//    11000000 | 00001100  ->  11001100
	return preservedBits | newBits
}

// TurnOn turns on all managed outputs (that is, outputs
// up to pf.maxSwitches).
func (pf *PiFace) TurnOn() error {
	log.Printf("turn on all switches on %s", pf)
	outputs, err := pf.ReadOutputs()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSwitchTurnOn, err)
	}
	outputs = setLowerBits(outputs, 0xff, uint8(pf.maxSwitches))
	if err := pf.WriteOutputs(outputs); err != nil {
		return fmt.Errorf("%w: %v", ErrSwitchTurnOn, err)
	}
	return nil
}

func (pf *PiFace) TurnOff() error {
	log.Printf("turn off all switches on %s", pf)
	outputs, err := pf.ReadOutputs()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSwitchTurnOn, err)
	}
	outputs = setLowerBits(outputs, 0x00, uint8(pf.maxSwitches))
	if err := pf.WriteOutputs(outputs); err != nil {
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

	states := make([]bool, pf.maxSwitches)
	for i := 0; i < int(pf.maxSwitches); i++ {
		states[i] = getBit(outputs, uint8(i))
	}
	return states, nil
}

// IsDisabled returns false since PiFace collections are never disabled
func (pf *PiFace) IsDisabled() bool {
	return false
}
