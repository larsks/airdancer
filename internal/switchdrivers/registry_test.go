package switchdrivers

import (
	"testing"

	"github.com/larsks/airdancer/internal/switchcollection"
)

func TestDefaultRegistry_SwitchDrivers(t *testing.T) {
	// Test that all expected switch drivers are automatically registered
	drivers := ListDrivers()

	foundDrivers := make(map[string]bool)
	for _, driver := range drivers {
		foundDrivers[driver] = true
	}

	expectedDrivers := []string{"piface", "gpio", "dummy"}

	for _, expected := range expectedDrivers {
		if !foundDrivers[expected] {
			t.Errorf("Expected switch driver %s not found in registry", expected)
		}
	}
}

func TestSwitchDriverFactory_Integration(t *testing.T) {
	// Test dummy driver creation (safe for testing)
	config := map[string]interface{}{
		"switch-count": 4,
	}

	driver, err := Create("dummy", config)
	if err != nil {
		t.Fatalf("Failed to create dummy switch driver: %v", err)
	}

	if driver == nil {
		t.Error("Dummy switch driver should not be nil")
	}

	// Test that it implements the switchcollection.SwitchCollection interface
	_, ok := driver.(switchcollection.SwitchCollection)
	if !ok {
		t.Error("Dummy switch driver should implement switchcollection.SwitchCollection interface")
	}

	// Test driver functionality
	if driver.CountSwitches() != 4 {
		t.Errorf("Expected 4 switches, got %d", driver.CountSwitches())
	}
}

func TestValidateConfig(t *testing.T) {
	// Test valid dummy config
	validConfig := map[string]interface{}{
		"switch-count": 8,
	}

	err := ValidateConfig("dummy", validConfig)
	if err != nil {
		t.Errorf("Valid dummy config should not produce error: %v", err)
	}

	// Test invalid dummy config
	invalidConfig := map[string]interface{}{
		"switch-count": -1,
	}

	err = ValidateConfig("dummy", invalidConfig)
	if err == nil {
		t.Error("Invalid dummy config should produce error")
	}

	// Test unknown driver
	err = ValidateConfig("nonexistent", validConfig)
	if err == nil {
		t.Error("Unknown driver should produce error")
	}
}
