//go:build integration && gpio
// +build integration,gpio

package gpio

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestGPIOIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Check if GPIO character devices exist (modern GPIO interface)
	found := false
	for i := 0; i < 4; i++ { // Check for common gpiochip devices
		if _, err := os.Stat(fmt.Sprintf("/dev/gpiochip%d", i)); err == nil {
			found = true
			break
		}
	}
	if !found {
		t.Skip("GPIO character devices not found - GPIO hardware not available")
	}

	// Test with some common GPIO pins (adjust for your hardware)
	testPins := []string{"23", "24"}

	gpio, err := NewGPIOSwitchCollection(true, testPins)
	if err != nil {
		t.Skipf("Failed to initialize GPIO hardware (hardware may not be available): %v", err)
	}
	defer gpio.Close()

	if err := gpio.Init(); err != nil {
		t.Skipf("Failed to initialize GPIO (hardware may not be available): %v", err)
	}

	t.Run("turn_on_all_outputs", func(t *testing.T) {
		err := gpio.TurnOn()
		if err != nil {
			t.Errorf("Failed to turn on all GPIO outputs: %v", err)
		}

		// Give hardware time to respond
		time.Sleep(100 * time.Millisecond)

		// Verify all switches report as on
		count := gpio.CountSwitches()
		for i := uint(0); i < count; i++ {
			sw, err := gpio.GetSwitch(i)
			if err != nil {
				t.Errorf("Failed to get GPIO switch %d: %v", i, err)
				continue
			}
			state, err := sw.GetState()
			if err != nil {
				t.Errorf("Failed to get state of GPIO switch %d: %v", i, err)
				continue
			}
			if !state {
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
		count := gpio.CountSwitches()
		for i := uint(0); i < count; i++ {
			sw, err := gpio.GetSwitch(i)
			if err != nil {
				t.Errorf("Failed to get GPIO switch %d: %v", i, err)
				continue
			}
			state, err := sw.GetState()
			if err != nil {
				t.Errorf("Failed to get state of GPIO switch %d: %v", i, err)
				continue
			}
			if state {
				t.Errorf("GPIO switch %d should be off but reports on", i)
			}
		}
	})

	t.Run("individual_gpio_control", func(t *testing.T) {
		count := gpio.CountSwitches()
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
		state, err := sw.GetState()
		if err != nil {
			t.Errorf("Failed to get state of GPIO switch 0: %v", err)
		} else if !state {
			t.Errorf("GPIO switch 0 should be on but reports off")
		}

		// Turn off
		if err := sw.TurnOff(); err != nil {
			t.Errorf("Failed to turn off GPIO switch 0: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
		state, err = sw.GetState()
		if err != nil {
			t.Errorf("Failed to get state of GPIO switch 0: %v", err)
		} else if state {
			t.Errorf("GPIO switch 0 should be off but reports on")
		}

		// Test state changes (manual toggle since Toggle() method doesn't exist)
		initialState, err := sw.GetState()
		if err != nil {
			t.Errorf("Failed to get initial state of GPIO switch 0: %v", err)
		} else {
			// Manually toggle by turning on if off, or off if on
			if initialState {
				if err := sw.TurnOff(); err != nil {
					t.Errorf("Failed to turn off GPIO switch 0: %v", err)
				}
			} else {
				if err := sw.TurnOn(); err != nil {
					t.Errorf("Failed to turn on GPIO switch 0: %v", err)
				}
			}
			time.Sleep(50 * time.Millisecond)
			finalState, err := sw.GetState()
			if err != nil {
				t.Errorf("Failed to get final state of GPIO switch 0: %v", err)
			} else if finalState == initialState {
				t.Errorf("GPIO switch 0 state should have changed after manual toggle")
			}
		}
	})
}

func TestGPIOHardwareDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping hardware detection test in short mode")
	}

	t.Run("gpio_character_devices_exist", func(t *testing.T) {
		// Check if GPIO character devices are available (modern interface)
		found := false
		var availableChips []string

		for i := 0; i < 8; i++ { // Check more chips for thoroughness
			chipPath := fmt.Sprintf("/dev/gpiochip%d", i)
			if _, err := os.Stat(chipPath); err == nil {
				found = true
				availableChips = append(availableChips, chipPath)
			}
		}

		if !found {
			t.Skip("No GPIO character devices found - GPIO hardware not available")
		}

		t.Logf("Found GPIO character devices: %v", availableChips)
	})

	t.Run("gpio_chip_accessibility", func(t *testing.T) {
		// Check if we can access GPIO chip devices
		found := false

		for i := 0; i < 4; i++ {
			chipPath := fmt.Sprintf("/dev/gpiochip%d", i)
			if info, err := os.Stat(chipPath); err == nil {
				found = true
				t.Logf("GPIO chip %s found with permissions %v", chipPath, info.Mode())

				// Check if it's a character device
				if info.Mode()&os.ModeCharDevice == 0 {
					t.Logf("Warning: %s is not a character device", chipPath)
				}
			}
		}

		if !found {
			t.Skip("No accessible GPIO chip devices found")
		}
	})
}
