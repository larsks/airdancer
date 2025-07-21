package switchdrivers

import (
	"fmt"
	"sync"

	"github.com/larsks/airdancer/internal/switchcollection"
)

// Factory creates a switch collection from configuration
type Factory interface {
	CreateDriver(config map[string]interface{}) (switchcollection.SwitchCollection, error)
	ValidateConfig(config map[string]interface{}) error
}

// Registry manages driver factories
type Registry struct {
	drivers map[string]Factory
	mu      sync.RWMutex
}

// NewRegistry creates a new driver registry
func NewRegistry() *Registry {
	return &Registry{
		drivers: make(map[string]Factory),
	}
}

// Register adds a driver factory to the registry
func (r *Registry) Register(name string, factory Factory) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.drivers[name]; exists {
		return fmt.Errorf("driver %s already registered", name)
	}

	r.drivers[name] = factory
	return nil
}

// Create creates a switch collection using the specified driver
func (r *Registry) Create(driverName string, config map[string]interface{}) (switchcollection.SwitchCollection, error) {
	r.mu.RLock()
	factory, exists := r.drivers[driverName]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unknown driver: %s", driverName)
	}

	return factory.CreateDriver(config)
}

// ValidateConfig validates configuration for the specified driver
func (r *Registry) ValidateConfig(driverName string, config map[string]interface{}) error {
	r.mu.RLock()
	factory, exists := r.drivers[driverName]
	r.mu.RUnlock()

	if !exists {
		return fmt.Errorf("unknown driver: %s", driverName)
	}

	return factory.ValidateConfig(config)
}

// ListDrivers returns the names of all registered drivers
func (r *Registry) ListDrivers() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.drivers))
	for name := range r.drivers {
		names = append(names, name)
	}
	return names
}

// Default registry instance
var defaultRegistry = NewRegistry()

// Register adds a driver factory to the default registry
func Register(name string, factory Factory) error {
	return defaultRegistry.Register(name, factory)
}

// Create creates a switch collection using the default registry
func Create(driverName string, config map[string]interface{}) (switchcollection.SwitchCollection, error) {
	return defaultRegistry.Create(driverName, config)
}

// ValidateConfig validates configuration using the default registry
func ValidateConfig(driverName string, config map[string]interface{}) error {
	return defaultRegistry.ValidateConfig(driverName, config)
}

// ListDrivers returns the names of all registered drivers in the default registry
func ListDrivers() []string {
	return defaultRegistry.ListDrivers()
}
