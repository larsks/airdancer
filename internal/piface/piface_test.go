package piface

import (
	"errors"
	"fmt"
	"testing"

	"github.com/larsks/airdancer/internal/switchcollection"
)

// Test pure helper functions
func TestValidatePin(t *testing.T) {
	tests := []struct {
		name    string
		pin     uint8
		wantErr bool
		errType error
	}{
		{"valid pin 0", 0, false, nil},
		{"valid pin 7", 7, false, nil},
		{"valid pin middle", 3, false, nil},
		{"invalid pin 8", 8, true, ErrInvalidPin},
		{"invalid pin 255", 255, true, ErrInvalidPin},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePin(tt.pin)
			if tt.wantErr {
				if err == nil {
					t.Errorf("validatePin() expected error, got nil")
					return
				}
				if !errors.Is(err, tt.errType) {
					t.Errorf("validatePin() error = %v, want error type %v", err, tt.errType)
				}
			} else if err != nil {
				t.Errorf("validatePin() unexpected error = %v", err)
			}
		})
	}
}

func TestSetBit(t *testing.T) {
	tests := []struct {
		name   string
		value  uint8
		pin    uint8
		state  bool
		expect uint8
	}{
		{"set bit 0 on empty", 0x00, 0, true, 0x01},
		{"set bit 7 on empty", 0x00, 7, true, 0x80},
		{"set bit 3 on empty", 0x00, 3, true, 0x08},
		{"clear bit 0 on full", 0xFF, 0, false, 0xFE},
		{"clear bit 7 on full", 0xFF, 7, false, 0x7F},
		{"clear bit 3 on full", 0xFF, 3, false, 0xF7},
		{"set already set bit", 0x01, 0, true, 0x01},
		{"clear already clear bit", 0xFE, 0, false, 0xFE},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := setBit(tt.value, tt.pin, tt.state)
			if result != tt.expect {
				t.Errorf("setBit(%02x, %d, %t) = %02x, want %02x",
					tt.value, tt.pin, tt.state, result, tt.expect)
			}
		})
	}
}

func TestGetBit(t *testing.T) {
	tests := []struct {
		name   string
		value  uint8
		pin    uint8
		expect bool
	}{
		{"get bit 0 from 0x01", 0x01, 0, true},
		{"get bit 0 from 0x00", 0x00, 0, false},
		{"get bit 7 from 0x80", 0x80, 7, true},
		{"get bit 7 from 0x7F", 0x7F, 7, false},
		{"get bit 3 from 0x08", 0x08, 3, true},
		{"get bit 3 from 0xF7", 0xF7, 3, false},
		{"get bit from mixed pattern", 0xAA, 1, true},  // 10101010
		{"get bit from mixed pattern", 0xAA, 2, false}, // 10101010
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getBit(tt.value, tt.pin)
			if result != tt.expect {
				t.Errorf("getBit(%02x, %d) = %t, want %t",
					tt.value, tt.pin, result, tt.expect)
			}
		})
	}
}

// Mock PiFace for testing interface compliance without hardware
type mockPiFace struct {
	outputs uint8
}

func (m *mockPiFace) CountSwitches() uint {
	return NUMBER_OF_OUTPUTS
}

func (m *mockPiFace) GetSwitch(id uint) (switchcollection.Switch, error) {
	if id > 7 {
		return nil, ErrInvalidSwitchID
	}
	return &mockPiFaceOutput{pf: m, pin: uint8(id)}, nil
}

func (m *mockPiFace) ListSwitches() []switchcollection.Switch {
	var switches []switchcollection.Switch
	for i := uint(0); i < NUMBER_OF_OUTPUTS; i++ {
		if sw, err := m.GetSwitch(i); err == nil {
			switches = append(switches, sw)
		}
	}
	return switches
}

func (m *mockPiFace) TurnOn() error           { m.outputs = 0xFF; return nil }
func (m *mockPiFace) TurnOff() error          { m.outputs = 0x00; return nil }
func (m *mockPiFace) GetState() (bool, error) { return m.outputs == 0xFF, nil }
func (m *mockPiFace) IsDisabled() bool        { return false }
func (m *mockPiFace) Init() error             { return nil }
func (m *mockPiFace) Close() error            { return nil }
func (m *mockPiFace) String() string          { return "mock-piface" }
func (m *mockPiFace) GetDetailedState() ([]bool, error) {
	states := make([]bool, NUMBER_OF_OUTPUTS)
	for i := range NUMBER_OF_OUTPUTS {
		states[i] = getBit(m.outputs, uint8(i))
	}
	return states, nil
}

type mockPiFaceOutput struct {
	pf  *mockPiFace
	pin uint8
}

func (m *mockPiFaceOutput) TurnOn() error {
	m.pf.outputs = setBit(m.pf.outputs, m.pin, true)
	return nil
}

func (m *mockPiFaceOutput) TurnOff() error {
	m.pf.outputs = setBit(m.pf.outputs, m.pin, false)
	return nil
}

func (m *mockPiFaceOutput) GetState() (bool, error) {
	return getBit(m.pf.outputs, m.pin), nil
}

func (m *mockPiFaceOutput) IsDisabled() bool {
	return false
}

func (m *mockPiFaceOutput) String() string {
	return fmt.Sprintf("mock-piface:%d", m.pin)
}

// Test SwitchCollection interface compliance using mock
func TestSwitchCollectionInterface(t *testing.T) {
	mock := &mockPiFace{}

	t.Run("CountSwitches", func(t *testing.T) {
		count := mock.CountSwitches()
		if count != NUMBER_OF_OUTPUTS {
			t.Errorf("CountSwitches() = %d, want %d", count, NUMBER_OF_OUTPUTS)
		}
	})

	t.Run("GetSwitch valid IDs", func(t *testing.T) {
		for i := uint(0); i < NUMBER_OF_OUTPUTS; i++ {
			sw, err := mock.GetSwitch(i)
			if err != nil {
				t.Errorf("GetSwitch(%d) unexpected error: %v", i, err)
				continue
			}
			if sw == nil {
				t.Errorf("GetSwitch(%d) returned nil switch", i)
			}
		}
	})

	t.Run("GetSwitch invalid ID", func(t *testing.T) {
		_, err := mock.GetSwitch(8)
		if err == nil {
			t.Error("GetSwitch(8) expected error, got nil")
		}
		if !errors.Is(err, ErrInvalidSwitchID) {
			t.Errorf("GetSwitch(8) error = %v, want %v", err, ErrInvalidSwitchID)
		}
	})

	t.Run("ListSwitches", func(t *testing.T) {
		switches := mock.ListSwitches()
		if len(switches) != int(NUMBER_OF_OUTPUTS) {
			t.Errorf("ListSwitches() returned %d switches, want %d",
				len(switches), NUMBER_OF_OUTPUTS)
		}

		// Verify all switches are valid (non-nil)
		for i, sw := range switches {
			if sw == nil {
				t.Errorf("ListSwitches()[%d] returned nil switch", i)
			}
		}
	})

	t.Run("TurnOn/TurnOff/GetState", func(t *testing.T) {
		// Test initial state
		_, err := mock.GetState()
		if err != nil {
			t.Fatalf("GetState() error: %v", err)
		}

		// Turn on all switches
		if err := mock.TurnOn(); err != nil {
			t.Fatalf("TurnOn() error: %v", err)
		}

		state, err := mock.GetState()
		if err != nil {
			t.Fatalf("GetState() after TurnOn() error: %v", err)
		}
		if !state {
			t.Error("GetState() after TurnOn() = false, want true")
		}

		// Turn off all switches
		if err := mock.TurnOff(); err != nil {
			t.Fatalf("TurnOff() error: %v", err)
		}

		state, err = mock.GetState()
		if err != nil {
			t.Fatalf("GetState() after TurnOff() error: %v", err)
		}
		if state {
			t.Error("GetState() after TurnOff() = true, want false")
		}
	})

	t.Run("GetDetailedState", func(t *testing.T) {
		// Turn off all first
		mock.TurnOff()

		// Turn on specific switches
		sw0, _ := mock.GetSwitch(0)
		sw3, _ := mock.GetSwitch(3)
		sw7, _ := mock.GetSwitch(7)

		sw0.TurnOn()
		sw3.TurnOn()
		sw7.TurnOn()

		states, err := mock.GetDetailedState()
		if err != nil {
			t.Fatalf("GetDetailedState() error: %v", err)
		}

		expected := []bool{true, false, false, true, false, false, false, true}
		if len(states) != len(expected) {
			t.Fatalf("GetDetailedState() returned %d states, want %d",
				len(states), len(expected))
		}

		for i, want := range expected {
			if states[i] != want {
				t.Errorf("GetDetailedState()[%d] = %t, want %t", i, states[i], want)
			}
		}
	})
}

// Test bit manipulation scenarios that would occur in real usage
func TestBitManipulationScenarios(t *testing.T) {
	t.Run("sequential bit operations", func(t *testing.T) {
		var value uint8 = 0x00

		// Set bits in sequence
		for pin := uint8(0); pin < 8; pin++ {
			value = setBit(value, pin, true)
			if !getBit(value, pin) {
				t.Errorf("After setting pin %d, getBit returned false", pin)
			}
		}

		if value != 0xFF {
			t.Errorf("After setting all bits, value = %02x, want 0xFF", value)
		}

		// Clear bits in reverse sequence
		for pin := uint8(7); pin < 8; pin-- { // pin is uint8, so < 8 handles wraparound
			value = setBit(value, pin, false)
			if getBit(value, pin) {
				t.Errorf("After clearing pin %d, getBit returned true", pin)
			}
		}

		if value != 0x00 {
			t.Errorf("After clearing all bits, value = %02x, want 0x00", value)
		}
	})

	t.Run("alternating pattern", func(t *testing.T) {
		var value uint8 = 0x00

		// Create alternating pattern (0xAA = 10101010)
		for pin := uint8(0); pin < 8; pin++ {
			value = setBit(value, pin, pin%2 == 1)
		}

		if value != 0xAA {
			t.Errorf("Alternating pattern result = %02x, want 0xAA", value)
		}

		// Verify pattern
		for pin := uint8(0); pin < 8; pin++ {
			expected := pin%2 == 1
			if getBit(value, pin) != expected {
				t.Errorf("Pin %d state = %t, want %t", pin, getBit(value, pin), expected)
			}
		}
	})
}
