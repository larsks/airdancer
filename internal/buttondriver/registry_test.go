package buttondriver

import (
	"testing"

	"github.com/larsks/airdancer/internal/buttondriver/common"
)

// MockButtonDriver for testing
type MockButtonDriver struct {
	events chan common.ButtonEvent
}

func (m *MockButtonDriver) Events() <-chan common.ButtonEvent {
	return m.events
}

func (m *MockButtonDriver) Start() error {
	return nil
}

func (m *MockButtonDriver) Stop() {
	close(m.events)
}

func (m *MockButtonDriver) AddButton(buttonSpec interface{}) error {
	return nil
}

func (m *MockButtonDriver) GetButtons() []string {
	return []string{"test"}
}

// MockFactory for testing
type MockFactory struct {
	driverConfig map[string]interface{}
}

func (m *MockFactory) CreateDriver(config map[string]interface{}) (common.ButtonDriver, error) {
	m.driverConfig = config
	return &MockButtonDriver{
		events: make(chan common.ButtonEvent, 10),
	}, nil
}

func (m *MockFactory) ParseButtonSpec(spec string) (interface{}, error) {
	return map[string]string{"spec": spec}, nil
}

func (m *MockFactory) ValidateConfig(config map[string]interface{}) error {
	return nil
}

func TestRegistry_Register(t *testing.T) {
	registry := NewRegistry()
	factory := &MockFactory{}

	err := registry.Register("test", factory)
	if err != nil {
		t.Fatalf("Failed to register driver: %v", err)
	}

	// Test duplicate registration
	err = registry.Register("test", factory)
	if err == nil {
		t.Error("Expected error for duplicate registration")
	}
}

func TestRegistry_CreateDriver(t *testing.T) {
	registry := NewRegistry()
	factory := &MockFactory{}

	err := registry.Register("test", factory)
	if err != nil {
		t.Fatalf("Failed to register driver: %v", err)
	}

	config := map[string]interface{}{
		"pull-mode":   "up",
		"debounce-ms": 50,
	}

	driver, err := registry.CreateDriver("test", config)
	if err != nil {
		t.Fatalf("Failed to create driver: %v", err)
	}

	if driver == nil {
		t.Error("Driver should not be nil")
	}

	// Check that config was passed to factory
	if factory.driverConfig["pull-mode"] != "up" {
		t.Errorf("Expected pull-mode 'up', got %v", factory.driverConfig["pull-mode"])
	}
}

func TestRegistry_CreateDriver_UnknownDriver(t *testing.T) {
	registry := NewRegistry()

	_, err := registry.CreateDriver("nonexistent", nil)
	if err == nil {
		t.Error("Expected error for unknown driver")
	}
}

func TestRegistry_ParseButtonSpec(t *testing.T) {
	registry := NewRegistry()
	factory := &MockFactory{}

	err := registry.Register("test", factory)
	if err != nil {
		t.Fatalf("Failed to register driver: %v", err)
	}

	spec, err := registry.ParseButtonSpec("test", "button1:GPIO16")
	if err != nil {
		t.Fatalf("Failed to parse button spec: %v", err)
	}

	specMap, ok := spec.(map[string]string)
	if !ok {
		t.Fatalf("Expected map[string]string, got %T", spec)
	}

	if specMap["spec"] != "button1:GPIO16" {
		t.Errorf("Expected spec 'button1:GPIO16', got %v", specMap["spec"])
	}
}

func TestRegistry_ListDrivers(t *testing.T) {
	registry := NewRegistry()
	factory := &MockFactory{}

	err := registry.Register("test1", factory)
	if err != nil {
		t.Fatalf("Failed to register driver: %v", err)
	}

	err = registry.Register("test2", factory)
	if err != nil {
		t.Fatalf("Failed to register driver: %v", err)
	}

	drivers := registry.ListDrivers()
	if len(drivers) != 2 {
		t.Errorf("Expected 2 drivers, got %d", len(drivers))
	}

	// Check that both drivers are in the list
	found := make(map[string]bool)
	for _, driver := range drivers {
		found[driver] = true
	}

	if !found["test1"] || !found["test2"] {
		t.Errorf("Expected both test1 and test2 drivers in list, got %v", drivers)
	}
}

func TestDefaultRegistry(t *testing.T) {
	// Test that GPIO and event drivers are automatically registered
	drivers := ListDrivers()

	foundGPIO := false
	foundEvent := false

	for _, driver := range drivers {
		if driver == "gpio" {
			foundGPIO = true
		}
		if driver == "event" {
			foundEvent = true
		}
	}

	if !foundGPIO {
		t.Error("GPIO driver not found in default registry")
	}

	if !foundEvent {
		t.Error("Event driver not found in default registry")
	}
}

func TestGPIOFactory_Integration(t *testing.T) {
	config := map[string]interface{}{
		"pull-mode":   "up",
		"debounce-ms": 25,
	}

	driver, err := CreateDriver("gpio", config)
	if err != nil {
		t.Fatalf("Failed to create GPIO driver: %v", err)
	}

	if driver == nil {
		t.Error("GPIO driver should not be nil")
	}

}

func TestEventFactory_Integration(t *testing.T) {
	config := map[string]interface{}{}

	driver, err := CreateDriver("event", config)
	if err != nil {
		t.Fatalf("Failed to create event driver: %v", err)
	}

	if driver == nil {
		t.Error("Event driver should not be nil")
	}

}
