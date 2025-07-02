//go:build integration && piface
// +build integration,piface

package piface

import (
	"testing"
	"time"
)

func TestPiFaceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Test requires actual PiFace hardware
	pf, err := NewPiFace(true, "/dev/spidev0.0")
	if err != nil {
		t.Fatalf("Failed to initialize PiFace hardware: %v", err)
	}
	defer pf.Close()

	if err := pf.Init(); err != nil {
		t.Fatalf("Failed to initialize PiFace: %v", err)
	}

	t.Run("turn_on_all_outputs", func(t *testing.T) {
		err := pf.TurnOn()
		if err != nil {
			t.Errorf("Failed to turn on all outputs: %v", err)
		}

		// Give hardware time to respond
		time.Sleep(100 * time.Millisecond)

		// Verify all switches report as on
		count := pf.GetSwitchCount()
		for i := uint(0); i < count; i++ {
			sw, err := pf.GetSwitch(i)
			if err != nil {
				t.Errorf("Failed to get switch %d: %v", i, err)
				continue
			}
			if !sw.IsOn() {
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
		count := pf.GetSwitchCount()
		for i := uint(0); i < count; i++ {
			sw, err := pf.GetSwitch(i)
			if err != nil {
				t.Errorf("Failed to get switch %d: %v", i, err)
				continue
			}
			if sw.IsOn() {
				t.Errorf("Switch %d should be off but reports on", i)
			}
		}
	})

	t.Run("individual_switch_control", func(t *testing.T) {
		count := pf.GetSwitchCount()
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
		if !sw.IsOn() {
			t.Errorf("Switch 0 should be on but reports off")
		}

		// Turn off
		if err := sw.TurnOff(); err != nil {
			t.Errorf("Failed to turn off switch 0: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
		if sw.IsOn() {
			t.Errorf("Switch 0 should be off but reports on")
		}

		// Toggle
		initialState := sw.IsOn()
		if err := sw.Toggle(); err != nil {
			t.Errorf("Failed to toggle switch 0: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
		if sw.IsOn() == initialState {
			t.Errorf("Switch 0 state should have changed after toggle")
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
			pf, err := NewPiFace(false, device) // Don't initialize yet
			if err == nil {
				t.Logf("Found PiFace-compatible SPI device: %s", device)
				foundDevice = true
				pf.Close()
				break
			}
		}

		if !foundDevice {
			t.Skip("No PiFace-compatible SPI devices found - hardware may not be available")
		}
	})
}
