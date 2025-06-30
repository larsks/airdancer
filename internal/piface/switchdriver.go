package piface

import (
	"fmt"
	"github.com/larsks/airdancer/internal/switchdriver"
)

type (
	PiFaceOutput struct {
		pf  *PiFace
		pin uint8
	}
)

func (pf *PiFace) String() string {
	return fmt.Sprintf("piface:%s", pf.spiPortName)
}

func (pf *PiFace) CountSwitches() uint {
	return NUMBER_OF_OUTPUTS
}

func (pf *PiFace) ListSwitches() []switchdriver.Switch {
	var switches []switchdriver.Switch
	for i := range NUMBER_OF_OUTPUTS {
		if sw, err := pf.GetSwitch(uint(i)); err == nil {
			switches = append(switches, sw)
		}
	}

	return switches
}

func (pf *PiFace) GetSwitch(id uint) (switchdriver.Switch, error) {
	if id > 7 {
		return nil, fmt.Errorf("invalid switch id")
	}
	return &PiFaceOutput{
		pf:  pf,
		pin: uint8(id),
	}, nil
}

func (pf *PiFace) TurnAllOn() error {
	for _, sw := range pf.ListSwitches() {
		if err := sw.TurnOn(); err != nil {
			return err
		}
	}
	return nil
}

func (pf *PiFace) TurnAllOff() error {
	for _, sw := range pf.ListSwitches() {
		if err := sw.TurnOff(); err != nil {
			return err
		}
	}
	return nil
}

func (pfo *PiFaceOutput) TurnOn() error {
	return pfo.pf.WriteOutput(pfo.pin, 1)
}

func (pfo *PiFaceOutput) TurnOff() error {
	return pfo.pf.WriteOutput(pfo.pin, 0)
}

func (pfo *PiFaceOutput) GetID() uint {
	return uint(pfo.pin)
}

func (pfo *PiFaceOutput) GetState() (bool, error) {
	val, err := pfo.pf.ReadOutput(pfo.pin)
	if err != nil {
		return false, err
	}

	return val != 0, nil
}

func (pfo *PiFaceOutput) String() string {
	return fmt.Sprintf("%s:%d", pfo.pf, pfo.pin)
}
