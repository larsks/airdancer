package buttondriver

import (
	"fmt"
	"sync"

	"github.com/larsks/airdancer/internal/buttondriver/common"
)

// Factory creates a button driver from configuration and parses button specifications
type Factory interface {
	// CreateDriver creates a new button driver instance with the given configuration
	CreateDriver(config map[string]interface{}) (common.ButtonDriver, error)

	// ParseButtonSpec parses a button specification string into a button spec object
	ParseButtonSpec(spec string) (interface{}, error)

	// ValidateConfig validates the driver configuration
	ValidateConfig(config map[string]interface{}) error
}

// Registry manages button driver factories
type Registry struct {
	drivers map[string]Factory
	mu      sync.RWMutex
}

// NewRegistry creates a new button driver registry
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
		return fmt.Errorf("button driver %s already registered", name)
	}

	r.drivers[name] = factory
	return nil
}

// CreateDriver creates a button driver using the specified driver type and configuration
func (r *Registry) CreateDriver(driverType string, config map[string]interface{}) (common.ButtonDriver, error) {
	r.mu.RLock()
	factory, exists := r.drivers[driverType]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unknown button driver: %s", driverType)
	}

	return factory.CreateDriver(config)
}

// ParseButtonSpec parses a button specification using the specified driver type
func (r *Registry) ParseButtonSpec(driverType string, spec string) (interface{}, error) {
	r.mu.RLock()
	factory, exists := r.drivers[driverType]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unknown button driver: %s", driverType)
	}

	return factory.ParseButtonSpec(spec)
}

// ValidateConfig validates configuration for the specified driver
func (r *Registry) ValidateConfig(driverType string, config map[string]interface{}) error {
	r.mu.RLock()
	factory, exists := r.drivers[driverType]
	r.mu.RUnlock()

	if !exists {
		return fmt.Errorf("unknown button driver: %s", driverType)
	}

	return factory.ValidateConfig(config)
}

// ListDrivers returns the names of all registered button drivers
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

// MustRegister adds a driver factory to the default registry and panics on error
func MustRegister(name string, factory Factory) {
	if err := Register(name, factory); err != nil {
		panic(fmt.Sprintf("failed to register button driver %s: %v", name, err))
	}
}

// CreateDriver creates a button driver using the default registry
func CreateDriver(driverType string, config map[string]interface{}) (common.ButtonDriver, error) {
	return defaultRegistry.CreateDriver(driverType, config)
}

// ParseButtonSpec parses a button specification using the default registry
func ParseButtonSpec(driverType string, spec string) (interface{}, error) {
	return defaultRegistry.ParseButtonSpec(driverType, spec)
}

// ValidateConfig validates configuration using the default registry
func ValidateConfig(driverType string, config map[string]interface{}) error {
	return defaultRegistry.ValidateConfig(driverType, config)
}

// ListDrivers returns the names of all registered button drivers in the default registry
func ListDrivers() []string {
	return defaultRegistry.ListDrivers()
}
