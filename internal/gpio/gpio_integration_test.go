//go:build integration && gpio
// +build integration,gpio

package gpio

import (
	"testing"
	"time"
)

func TestGPIOIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Test with some common GPIO pins (adjust for your hardware)
	testPins := []string{"23", "24"}

	gpio, err := NewGPIOSwitchCollection(true, testPins)
	if err != nil {
		t.Fatalf("Failed to initialize GPIO hardware: %v", err)
	}
	defer gpio.Close()

	if err := gpio.Init(); err != nil {
		t.Fatalf("Failed to initialize GPIO: %v", err)
	}

	t.Run("turn_on_all_outputs", func(t *testing.T) {
		err := gpio.TurnOn()
		if err != nil {
			t.Errorf("Failed to turn on all GPIO outputs: %v", err)
		}

		// Give hardware time to respond
		time.Sleep(100 * time.Millisecond)

		// Verify all switches report as on
		count := gpio.GetSwitchCount()
		for i := uint(0); i < count; i++ {
			sw, err := gpio.GetSwitch(i)
			if err != nil {
				t.Errorf("Failed to get GPIO switch %d: %v", i, err)
				continue
			}
			if !sw.IsOn() {
				t.Errorf("GPIO switch %d should be on but reports off", i)
			}
		}
	})

	t.Run("turn_off_all_outputs", func(t *testing.T) {
		err := gpio.TurnOff()
		if err != nil {
			t.Errorf("Failed to turn off all GPIO outputs: %v", err)
		}

		time.Sleep(100 * time.Millisecond)

		// Verify all switches report as off
		count := gpio.GetSwitchCount()
		for i := uint(0); i < count; i++ {
			sw, err := gpio.GetSwitch(i)
			if err != nil {
				t.Errorf("Failed to get GPIO switch %d: %v", i, err)
				continue
			}
			if sw.IsOn() {
				t.Errorf("GPIO switch %d should be off but reports on", i)
			}
		}
	})

	t.Run("individual_gpio_control", func(t *testing.T) {
		count := gpio.GetSwitchCount()
		if count == 0 {
			t.Skip("No GPIO switches available for testing")
		}

		// Test first GPIO pin
		sw, err := gpio.GetSwitch(0)
		if err != nil {
			t.Fatalf("Failed to get GPIO switch 0: %v", err)
		}

		// Turn on
		if err := sw.TurnOn(); err != nil {
			t.Errorf("Failed to turn on GPIO switch 0: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
		if !sw.IsOn() {
			t.Errorf("GPIO switch 0 should be on but reports off")
		}

		// Turn off
		if err := sw.TurnOff(); err != nil {
			t.Errorf("Failed to turn off GPIO switch 0: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
		if sw.IsOn() {
			t.Errorf("GPIO switch 0 should be off but reports on")
		}

		// Toggle
		initialState := sw.IsOn()
		if err := sw.Toggle(); err != nil {
			t.Errorf("Failed to toggle GPIO switch 0: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
		if sw.IsOn() == initialState {
			t.Errorf("GPIO switch 0 state should have changed after toggle")
		}
	})
}

func TestGPIOHardwareDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping hardware detection test in short mode")
	}

	t.Run("gpio_sysfs_exists", func(t *testing.T) {
		// Check if GPIO sysfs interface is available
		testPin := "18" // Common GPIO pin

		gpio, err := NewGPIOSwitchCollection(false, []string{testPin})
		if err != nil {
			t.Skipf("GPIO hardware not available: %v", err)
		}
		defer gpio.Close()

		t.Logf("GPIO sysfs interface appears to be available")
	})

	t.Run("gpio_permissions", func(t *testing.T) {
		// Test if we have permissions to control GPIO
		testPin := "18"

		gpio, err := NewGPIOSwitchCollection(true, []string{testPin})
		if err != nil {
			t.Skipf("GPIO permissions insufficient or hardware not available: %v", err)
		}
		defer gpio.Close()

		// Try to initialize
		if err := gpio.Init(); err != nil {
			t.Errorf("Failed to initialize GPIO (permissions issue?): %v", err)
		}

		t.Logf("GPIO permissions appear to be sufficient")
	})
}
