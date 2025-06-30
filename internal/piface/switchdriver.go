package piface

import (
	"fmt"
	"log"

	"github.com/larsks/airdancer/internal/switchcollection"
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
		return nil, fmt.Errorf("invalid switch id")
	}
	return &PiFaceOutput{
		pf:  pf,
		pin: uint8(id),
	}, nil
}

func (pf *PiFace) TurnOn() error {
	log.Printf("turn on all switches on %s", pf)
	return pf.WriteOutputs(0xff)
}

func (pf *PiFace) TurnOff() error {
	log.Printf("turn off all switches on %s", pf)
	return pf.WriteOutputs(0x0)
}

func (pfo *PiFaceOutput) TurnOn() error {
	log.Printf("turn on output %s", pfo)
	return pfo.pf.WriteOutput(pfo.pin, 1)
}

func (pfo *PiFaceOutput) TurnOff() error {
	log.Printf("turn off output %s", pfo)
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
