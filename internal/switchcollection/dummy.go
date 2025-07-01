package switchcollection

import (
	"fmt"
	"log"
	"sync"
)

// DummySwitch represents a single virtual switch for testing
type DummySwitch struct {
	id    uint
	state bool
	mutex sync.RWMutex
}

// DummySwitchCollection implements SwitchCollection for testing
type DummySwitchCollection struct {
	switches []Switch
	mutex    sync.RWMutex
}

// NewDummySwitchCollection creates a new dummy switch collection with specified number of switches
func NewDummySwitchCollection(switchCount uint) *DummySwitchCollection {
	switches := make([]Switch, switchCount)
	for i := uint(0); i < switchCount; i++ {
		switches[i] = &DummySwitch{
			id:    i,
			state: false,
		}
	}

	return &DummySwitchCollection{
		switches: switches,
	}
}

// Init initializes the dummy driver (no-op for dummy)
func (dsc *DummySwitchCollection) Init() error {
	log.Printf("initializing dummy switch collection with %d switches", len(dsc.switches))
	return nil
}

// Close closes the dummy driver (no-op for dummy)
func (dsc *DummySwitchCollection) Close() error {
	log.Printf("closing dummy switch collection")
	return nil
}

// CountSwitches returns the number of switches
func (dsc *DummySwitchCollection) CountSwitches() uint {
	dsc.mutex.RLock()
	defer dsc.mutex.RUnlock()
	return uint(len(dsc.switches))
}

// ListSwitches returns all switches
func (dsc *DummySwitchCollection) ListSwitches() []Switch {
	dsc.mutex.RLock()
	defer dsc.mutex.RUnlock()
	return dsc.switches
}

// GetSwitch returns a specific switch by ID
func (dsc *DummySwitchCollection) GetSwitch(id uint) (Switch, error) {
	dsc.mutex.RLock()
	defer dsc.mutex.RUnlock()

	if id >= uint(len(dsc.switches)) {
		return nil, fmt.Errorf("invalid switch id %d", id)
	}
	return dsc.switches[id], nil
}

// TurnOn turns on all switches
func (dsc *DummySwitchCollection) TurnOn() error {
	dsc.mutex.Lock()
	defer dsc.mutex.Unlock()

	log.Printf("turning on all dummy switches")
	for _, sw := range dsc.switches {
		if err := sw.TurnOn(); err != nil {
			return err
		}
	}
	return nil
}

// TurnOff turns off all switches
func (dsc *DummySwitchCollection) TurnOff() error {
	dsc.mutex.Lock()
	defer dsc.mutex.Unlock()

	log.Printf("turning off all dummy switches")
	for _, sw := range dsc.switches {
		if err := sw.TurnOff(); err != nil {
			return err
		}
	}
	return nil
}

// GetState returns true if all switches are on
func (dsc *DummySwitchCollection) GetState() (bool, error) {
	dsc.mutex.RLock()
	defer dsc.mutex.RUnlock()

	for _, sw := range dsc.switches {
		state, err := sw.GetState()
		if err != nil {
			return false, err
		}
		if !state {
			return false, nil
		}
	}
	return true, nil
}

// GetDetailedState returns the state of all switches
func (dsc *DummySwitchCollection) GetDetailedState() ([]bool, error) {
	dsc.mutex.RLock()
	defer dsc.mutex.RUnlock()

	states := make([]bool, len(dsc.switches))
	for i, sw := range dsc.switches {
		state, err := sw.GetState()
		if err != nil {
			return nil, err
		}
		states[i] = state
	}
	return states, nil
}

// String returns a string representation
func (dsc *DummySwitchCollection) String() string {
	return fmt.Sprintf("dummy switch collection with %d switches", len(dsc.switches))
}

// Individual DummySwitch methods

// TurnOn turns on the switch
func (ds *DummySwitch) TurnOn() error {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()

	log.Printf("turning on dummy switch %d", ds.id)
	ds.state = true
	return nil
}

// TurnOff turns off the switch
func (ds *DummySwitch) TurnOff() error {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()

	log.Printf("turning off dummy switch %d", ds.id)
	ds.state = false
	return nil
}

// GetState returns the current state of the switch
func (ds *DummySwitch) GetState() (bool, error) {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()

	return ds.state, nil
}

// String returns a string representation of the switch
func (ds *DummySwitch) String() string {
	return fmt.Sprintf("dummy:%d", ds.id)
}
