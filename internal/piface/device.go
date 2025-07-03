package piface

import (
	"fmt"
	"log"

	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/spi"
	"periph.io/x/conn/v3/spi/spireg"
	"periph.io/x/host/v3"
)

// MCP23017 register addresses
const (
	GPIOA  = 0x12 // GPIO port A register
	GPIOB  = 0x13 // GPIO port B register
	IODIRA = 0x00 // I/O direction register A
	IODIRB = 0x01 // I/O direction register B
	IOCON  = 10   // I/O config
	GPPUA  = 12   // Port A pullups
	GPPUB  = 13   // Port B pullups
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

// Helper functions for bit operations and validation
func validatePin(pin uint8) error {
	if pin > 7 {
		return fmt.Errorf("%w: %d (must be 0-7)", ErrInvalidPin, pin)
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
		return nil, fmt.Errorf("%w: %v", ErrPeriphInitFailed, err)
	}

	// Open SPI port
	spiPort, err := spireg.Open(spiPortName)
	if err != nil {
		return nil, fmt.Errorf("%w %s: %v", ErrSPIPortOpen, spiPortName, err)
	}
	// Configure SPI connection with proper frequency units
	spiConn, err := spiPort.Connect(1*physic.MegaHertz, spi.Mode0, 8) // 1MHz, Mode 0, 8 bits
	if err != nil {
		spiPort.Close() //nolint:errcheck
		return nil, fmt.Errorf("%w: %v", ErrSPIConnect, err)
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
			return fmt.Errorf("%w (%s): %v", ErrRegisterWrite, step.desc, err)
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
		return fmt.Errorf("%w 0x%02x: %v", ErrRegisterWrite, reg, err)
	}
	return nil
}

func (pf *PiFace) readRegister(reg uint8) (uint8, error) {
	// Hardware CS is handled automatically by the SPI subsystem
	write := []byte{OPCODE_READ, reg, 0x00}
	read := make([]byte, len(write))

	if err := pf.spiConn.Tx(write, read); err != nil {
		return 0, fmt.Errorf("%w 0x%02x: %v", ErrRegisterRead, reg, err)
	}

	return read[2], nil // The third byte contains the register value
}
