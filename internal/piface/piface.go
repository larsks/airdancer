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

func NewPiFace(offOnClose bool, spiPortName string) (*PiFace, error) {
	// Open SPI port
	spiPort, err := spireg.Open(spiPortName)
	if err != nil {
		return nil, fmt.Errorf("failed to open SPI port %s: %s", spiPortName, err)
	}
	// Configure SPI connection with proper frequency units
	spiConn, err := spiPort.Connect(1*physic.MegaHertz, spi.Mode0, 8) // 1MHz, Mode 0, 8 bits
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SPI: %v", err)
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
	initVars := map[uint8]uint8{
		IOCON:  8,
		IODIRA: 0,
		IODIRB: 0xff,
		GPPUB:  0xff,
	}

	log.Printf("initializing piface %s", pf)
	for k, v := range initVars {
		if err := pf.writeRegister(k, v); err != nil {
			return fmt.Errorf("failed to initialize piface: %w", err)
		}
	}

	return nil
}

func (pf *PiFace) Close() error {
	if pf.offOnClose {
		_ = pf.TurnOff()
	}
	return pf.spiPort.Close()
}

func (pf *PiFace) writeRegister(reg, value uint8) error {
	// Hardware CS is handled automatically by the SPI subsystem
	write := []byte{OPCODE_WRITE, reg, value}
	read := make([]byte, len(write))

	return pf.spiConn.Tx(write, read)
}

func (pf *PiFace) readRegister(reg uint8) (uint8, error) {
	// Hardware CS is handled automatically by the SPI subsystem
	write := []byte{OPCODE_READ, reg, 0x00}
	read := make([]byte, len(write))

	if err := pf.spiConn.Tx(write, read); err != nil {
		return 0, err
	}

	return read[2], nil // The third byte contains the register value
}

func (pf *PiFace) ReadInputs() (uint8, error) {
	val, err := pf.readRegister(GPIOB)
	if err != nil {
		return 0, err
	}

	return val ^ 0xFF, nil
}

func (pf *PiFace) ReadInput(pin uint8) (uint8, error) {
	vec, err := pf.ReadInputs()
	if err != nil {
		return 0, err
	}

	return (vec >> pin) & 0x1, nil
}

func (pf *PiFace) WriteOutputs(val uint8) error {
	return pf.writeRegister(GPIOA, val)
}

func (pf *PiFace) WriteOutput(pin uint8, val uint8) error {
	if val > 1 {
		return fmt.Errorf("value must be 0 or 1")
	}
	outputs, err := pf.ReadOutputs()
	if err != nil {
		return err
	}

	if val == 1 {
		outputs |= (1 << pin)
	} else {
		outputs &^= (1 << pin)
	}

	return pf.WriteOutputs(outputs)
}

func (pf *PiFace) ReadOutputs() (uint8, error) {
	return pf.readRegister(GPIOA)
}

func (pf *PiFace) ReadOutput(pin uint8) (uint8, error) {
	val, err := pf.ReadOutputs()
	if err != nil {
		return 0, err
	}

	return (val >> pin) & 1, nil
}

func init() {
	// Initialize periph.io host
	if _, err := host.Init(); err != nil {
		log.Fatal("Failed to initialize periph.io:", err)
	}
}
