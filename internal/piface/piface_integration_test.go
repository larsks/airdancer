//go:build integration && piface
// +build integration,piface

package piface

import (
	"os"
	"testing"
	"time"
)

func TestPiFaceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Check if SPI device exists before attempting to use it
	spiDevice := "/dev/spidev0.0"
	if _, err := os.Stat(spiDevice); os.IsNotExist(err) {
		t.Skipf("SPI device %s not found - PiFace hardware not available", spiDevice)
	}

	// Test requires actual PiFace hardware
	pf, err := NewPiFace(true, spiDevice)
	if err != nil {
		t.Skipf("Failed to initialize PiFace hardware (hardware may not be available): %v", err)
	}
	defer pf.Close()

	if err := pf.Init(); err != nil {
		t.Skipf("Failed to initialize PiFace (hardware may not be available): %v", err)
	}

	t.Run("turn_on_all_outputs", func(t *testing.T) {
		err := pf.TurnOn()
		if err != nil {
			t.Errorf("Failed to turn on all outputs: %v", err)
		}

		// Give hardware time to respond
		time.Sleep(100 * time.Millisecond)

		// Verify all switches report as on
		count := pf.CountSwitches()
		for i := uint(0); i < count; i++ {
			sw, err := pf.GetSwitch(i)
			if err != nil {
				t.Errorf("Failed to get switch %d: %v", i, err)
				continue
			}
			state, err := sw.GetState()
			if err != nil {
				t.Errorf("Failed to get state of switch %d: %v", i, err)
				continue
			}
			if !state {
				t.Errorf("Switch %d should be on but reports off", i)
			}
		}
	})

	t.Run("turn_off_all_outputs", func(t *testing.T) {
		err := pf.TurnOff()
		if err != nil {
			t.Errorf("Failed to turn off all outputs: %v", err)
		}

		time.Sleep(100 * time.Millisecond)

		// Verify all switches report as off
		count := pf.CountSwitches()
		for i := uint(0); i < count; i++ {
			sw, err := pf.GetSwitch(i)
			if err != nil {
				t.Errorf("Failed to get switch %d: %v", i, err)
				continue
			}
			state, err := sw.GetState()
			if err != nil {
				t.Errorf("Failed to get state of switch %d: %v", i, err)
				continue
			}
			if state {
				t.Errorf("Switch %d should be off but reports on", i)
			}
		}
	})

	t.Run("individual_switch_control", func(t *testing.T) {
		count := pf.CountSwitches()
		if count == 0 {
			t.Skip("No switches available for testing")
		}

		// Test first switch
		sw, err := pf.GetSwitch(0)
		if err != nil {
			t.Fatalf("Failed to get switch 0: %v", err)
		}

		// Turn on
		if err := sw.TurnOn(); err != nil {
			t.Errorf("Failed to turn on switch 0: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
		state, err := sw.GetState()
		if err != nil {
			t.Errorf("Failed to get state of switch 0: %v", err)
		} else if !state {
			t.Errorf("Switch 0 should be on but reports off")
		}

		// Turn off
		if err := sw.TurnOff(); err != nil {
			t.Errorf("Failed to turn off switch 0: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
		state, err = sw.GetState()
		if err != nil {
			t.Errorf("Failed to get state of switch 0: %v", err)
		} else if state {
			t.Errorf("Switch 0 should be off but reports on")
		}

		// Test state changes (manual toggle since Toggle() method doesn't exist)
		initialState, err := sw.GetState()
		if err != nil {
			t.Errorf("Failed to get initial state of switch 0: %v", err)
		} else {
			// Manually toggle by turning on if off, or off if on
			if initialState {
				if err := sw.TurnOff(); err != nil {
					t.Errorf("Failed to turn off switch 0: %v", err)
				}
			} else {
				if err := sw.TurnOn(); err != nil {
					t.Errorf("Failed to turn on switch 0: %v", err)
				}
			}
			time.Sleep(50 * time.Millisecond)
			finalState, err := sw.GetState()
			if err != nil {
				t.Errorf("Failed to get final state of switch 0: %v", err)
			} else if finalState == initialState {
				t.Errorf("Switch 0 state should have changed after manual toggle")
			}
		}
	})
}

func TestPiFaceHardwareDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping hardware detection test in short mode")
	}

	t.Run("spi_device_exists", func(t *testing.T) {
		// Test common SPI device paths
		devices := []string{"/dev/spidev0.0", "/dev/spidev0.1"}

		foundDevice := false
		for _, device := range devices {
			if _, err := os.Stat(device); err == nil {
				t.Logf("Found SPI device: %s", device)
				foundDevice = true
				break
			}
		}

		if !foundDevice {
			t.Skip("No SPI devices found - PiFace hardware not available")
		}
	})
}
