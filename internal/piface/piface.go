package piface

import (
	"fmt"
	"log"

	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/spi"
	"periph.io/x/conn/v3/spi/spireg"
	"periph.io/x/host/v3"

	"github.com/larsks/airdancer/internal/switchcollection"
)



// MCP23017 register addresses
const (
	NUMBER_OF_OUTPUTS = 8
	GPIOA             = 0x12 // GPIO port A register
	GPIOB             = 0x13 // GPIO port B register
	IODIRA            = 0x00 // I/O direction register A
	IODIRB            = 0x01 // I/O direction register B
	IOCON             = 10   // I/O config
	GPPUA             = 12   // Port A pullups
	GPPUB             = 13   // Port B pullups
)

// MCP23017 SPI opcodes
const (
	OPCODE_WRITE = 0x40
	OPCODE_READ  = 0x41
)

type PiFace struct {
	spiPortName string
	spiPort     spi.PortCloser
	spiConn     spi.Conn
	offOnClose  bool
}

type PiFaceOutput struct {
	pf  *PiFace
	pin uint8
}

// Helper functions for bit operations and validation
func validatePin(pin uint8) error {
	if pin > 7 {
		return fmt.Errorf("invalid pin %d: must be 0-7", pin)
	}
	return nil
}

func setBit(value uint8, pin uint8, state bool) uint8 {
	if state {
		return value | (1 << pin)
	}
	return value &^ (1 << pin)
}

func getBit(value uint8, pin uint8) bool {
	return (value>>pin)&1 != 0
}

func NewPiFace(offOnClose bool, spiPortName string) (*PiFace, error) {
	// Initialize periph.io host
	if _, err := host.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize periph.io: %w", err)
	}

	// Open SPI port
	spiPort, err := spireg.Open(spiPortName)
	if err != nil {
		return nil, fmt.Errorf("failed to open SPI port %s: %w", spiPortName, err)
	}
	// Configure SPI connection with proper frequency units
	spiConn, err := spiPort.Connect(1*physic.MegaHertz, spi.Mode0, 8) // 1MHz, Mode 0, 8 bits
	if err != nil {
		spiPort.Close() // Clean up on error
		return nil, fmt.Errorf("failed to connect to SPI: %w", err)
	}
	log.Printf("opened piface device at %s", spiPortName)

	return &PiFace{
		spiPortName: spiPortName,
		spiPort:     spiPort,
		spiConn:     spiConn,
		offOnClose:  offOnClose,
	}, nil
}

func (pf *PiFace) Init() error {
	log.Printf("initializing piface %s", pf)
	
	initSequence := []struct {
		reg   uint8
		value uint8
		desc  string
	}{
		{IOCON, 0x08, "configure IOCON"},
		{IODIRA, 0x00, "set port A as outputs"},
		{IODIRB, 0xFF, "set port B as inputs"},
		{GPPUB, 0xFF, "enable port B pullups"},
	}
	
	for _, step := range initSequence {
		if err := pf.writeRegister(step.reg, step.value); err != nil {
			return fmt.Errorf("failed to %s: %w", step.desc, err)
		}
	}
	
	return nil
}

func (pf *PiFace) Close() error {
	if pf.offOnClose {
		if err := pf.TurnOff(); err != nil {
			log.Printf("warning: failed to turn off outputs during close: %v", err)
		}
	}
	return pf.spiPort.Close()
}

func (pf *PiFace) writeRegister(reg, value uint8) error {
	// Hardware CS is handled automatically by the SPI subsystem
	write := []byte{OPCODE_WRITE, reg, value}
	read := make([]byte, len(write))

	if err := pf.spiConn.Tx(write, read); err != nil {
		return fmt.Errorf("failed to write register 0x%02x: %w", reg, err)
	}
	return nil
}

func (pf *PiFace) readRegister(reg uint8) (uint8, error) {
	// Hardware CS is handled automatically by the SPI subsystem
	write := []byte{OPCODE_READ, reg, 0x00}
	read := make([]byte, len(write))

	if err := pf.spiConn.Tx(write, read); err != nil {
		return 0, fmt.Errorf("failed to read register 0x%02x: %w", reg, err)
	}

	return read[2], nil // The third byte contains the register value
}

func (pf *PiFace) ReadInputs() (uint8, error) {
	val, err := pf.readRegister(GPIOB)
	if err != nil {
		return 0, fmt.Errorf("failed to read inputs: %w", err)
	}

	return val ^ 0xFF, nil
}

func (pf *PiFace) ReadInput(pin uint8) (uint8, error) {
	if err := validatePin(pin); err != nil {
		return 0, fmt.Errorf("failed to read input: %w", err)
	}

	vec, err := pf.ReadInputs()
	if err != nil {
		return 0, fmt.Errorf("failed to read input pin %d: %w", pin, err)
	}

	return (vec >> pin) & 0x1, nil
}

func (pf *PiFace) WriteOutputs(val uint8) error {
	if err := pf.writeRegister(GPIOA, val); err != nil {
		return fmt.Errorf("failed to write outputs: %w", err)
	}
	return nil
}

func (pf *PiFace) WriteOutput(pin uint8, val uint8) error {
	if err := validatePin(pin); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}
	if val > 1 {
		return fmt.Errorf("invalid output value %d: must be 0 or 1", val)
	}
	
	outputs, err := pf.ReadOutputs()
	if err != nil {
		return fmt.Errorf("failed to write output pin %d: %w", pin, err)
	}

	newOutputs := setBit(outputs, pin, val == 1)
	if err := pf.WriteOutputs(newOutputs); err != nil {
		return fmt.Errorf("failed to write output pin %d: %w", pin, err)
	}
	
	return nil
}

func (pf *PiFace) ReadOutputs() (uint8, error) {
	val, err := pf.readRegister(GPIOA)
	if err != nil {
		return 0, fmt.Errorf("failed to read outputs: %w", err)
	}
	return val, nil
}

func (pf *PiFace) ReadOutput(pin uint8) (uint8, error) {
	if err := validatePin(pin); err != nil {
		return 0, fmt.Errorf("failed to read output: %w", err)
	}

	val, err := pf.ReadOutputs()
	if err != nil {
		return 0, fmt.Errorf("failed to read output pin %d: %w", pin, err)
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
		return nil, fmt.Errorf("invalid switch id %d: must be 0-7", id)
	}
	return &PiFaceOutput{
		pf:  pf,
		pin: uint8(id),
	}, nil
}

func (pf *PiFace) TurnOn() error {
	log.Printf("turn on all switches on %s", pf)
	if err := pf.WriteOutputs(0xff); err != nil {
		return fmt.Errorf("failed to turn on all switches: %w", err)
	}
	return nil
}

func (pf *PiFace) TurnOff() error {
	log.Printf("turn off all switches on %s", pf)
	if err := pf.WriteOutputs(0x0); err != nil {
		return fmt.Errorf("failed to turn off all switches: %w", err)
	}
	return nil
}

func (pf *PiFace) GetState() (bool, error) {
	outputs, err := pf.ReadOutputs()
	if err != nil {
		return false, fmt.Errorf("failed to get state: %w", err)
	}
	return outputs == 0xFF, nil
}

func (pf *PiFace) GetDetailedState() ([]bool, error) {
	outputs, err := pf.ReadOutputs()
	if err != nil {
		return nil, fmt.Errorf("failed to get detailed state: %w", err)
	}

	states := make([]bool, NUMBER_OF_OUTPUTS)
	for i := 0; i < NUMBER_OF_OUTPUTS; i++ {
		states[i] = getBit(outputs, uint8(i))
	}
	return states, nil
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
		return fmt.Errorf("failed to turn %s output %d: %w", action, pfo.pin, err)
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
		return false, fmt.Errorf("failed to get state for output %d: %w", pfo.pin, err)
	}

	return val != 0, nil
}

func (pfo *PiFaceOutput) String() string {
	return fmt.Sprintf("%s:%d", pfo.pf, pfo.pin)
}
