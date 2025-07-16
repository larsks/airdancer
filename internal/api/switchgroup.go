package api

import (
	"fmt"

	"github.com/larsks/airdancer/internal/switchcollection"
)

// SwitchGroup represents a named group of switches that implements SwitchCollection.
type SwitchGroup struct {
	name     string
	switches map[string]*ResolvedSwitch
}

// NewSwitchGroup creates a new SwitchGroup.
func NewSwitchGroup(name string, switches map[string]*ResolvedSwitch) *SwitchGroup {
	return &SwitchGroup{
		name:     name,
		switches: switches,
	}
}

// TurnOn turns on all switches in the group.
func (sg *SwitchGroup) TurnOn() error {
	var errors []error
	for switchName, resolvedSwitch := range sg.switches {
		if err := resolvedSwitch.Switch.TurnOn(); err != nil {
			errors = append(errors, fmt.Errorf("switch %s: %w", switchName, err))
		}
	}
	if len(errors) > 0 {
		return fmt.Errorf("errors turning on group %s: %v", sg.name, errors)
	}
	return nil
}

// TurnOff turns off all switches in the group.
func (sg *SwitchGroup) TurnOff() error {
	var errors []error
	for switchName, resolvedSwitch := range sg.switches {
		if err := resolvedSwitch.Switch.TurnOff(); err != nil {
			errors = append(errors, fmt.Errorf("switch %s: %w", switchName, err))
		}
	}
	if len(errors) > 0 {
		return fmt.Errorf("errors turning off group %s: %v", sg.name, errors)
	}
	return nil
}

// GetState returns true if all switches in the group are on.
func (sg *SwitchGroup) GetState() (bool, error) {
	for switchName, resolvedSwitch := range sg.switches {
		state, err := resolvedSwitch.Switch.GetState()
		if err != nil {
			return false, fmt.Errorf("failed to get state for switch %s in group %s: %w", switchName, sg.name, err)
		}
		if !state {
			return false, nil
		}
	}
	return true, nil
}

// String returns the name of the group.
func (sg *SwitchGroup) String() string {
	return sg.name
}

// CountSwitches returns the number of switches in the group.
func (sg *SwitchGroup) CountSwitches() uint {
	return uint(len(sg.switches))
}

// ListSwitches returns all switches in the group.
func (sg *SwitchGroup) ListSwitches() []switchcollection.Switch {
	switches := make([]switchcollection.Switch, 0, len(sg.switches))
	for _, resolvedSwitch := range sg.switches {
		switches = append(switches, resolvedSwitch.Switch)
	}
	return switches
}

// GetSwitch returns a switch by index (order not guaranteed).
func (sg *SwitchGroup) GetSwitch(id uint) (switchcollection.Switch, error) {
	if id >= uint(len(sg.switches)) {
		return nil, fmt.Errorf("switch index %d out of range for group %s (max: %d)", id, sg.name, len(sg.switches)-1)
	}

	i := uint(0)
	for _, resolvedSwitch := range sg.switches {
		if i == id {
			return resolvedSwitch.Switch, nil
		}
		i++
	}

	return nil, fmt.Errorf("switch index %d not found in group %s", id, sg.name)
}

// GetDetailedState returns the state of all switches in the group.
func (sg *SwitchGroup) GetDetailedState() ([]bool, error) {
	states := make([]bool, 0, len(sg.switches))
	for switchName, resolvedSwitch := range sg.switches {
		state, err := resolvedSwitch.Switch.GetState()
		if err != nil {
			return nil, fmt.Errorf("failed to get state for switch %s in group %s: %w", switchName, sg.name, err)
		}
		states = append(states, state)
	}
	return states, nil
}

// Init initializes the switch group (no-op since switches are already initialized).
func (sg *SwitchGroup) Init() error {
	return nil
}

// Close closes the switch group (no-op since switches are managed by their collections).
func (sg *SwitchGroup) Close() error {
	return nil
}

// GetSwitches returns the map of switches in the group for status reporting.
func (sg *SwitchGroup) GetSwitches() map[string]*ResolvedSwitch {
	return sg.switches
}
