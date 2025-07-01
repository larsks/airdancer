package switchcollection

import (
	"testing"
)

func TestDummySwitchCollection(t *testing.T) {
	// Test creating a dummy switch collection
	switchCount := uint(4)
	dsc := NewDummySwitchCollection(switchCount)

	// Test Init
	if err := dsc.Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// Test CountSwitches
	if count := dsc.CountSwitches(); count != switchCount {
		t.Errorf("CountSwitches() = %d, want %d", count, switchCount)
	}

	// Test ListSwitches
	switches := dsc.ListSwitches()
	if len(switches) != int(switchCount) {
		t.Errorf("ListSwitches() returned %d switches, want %d", len(switches), switchCount)
	}

	// Test GetSwitch
	sw, err := dsc.GetSwitch(1)
	if err != nil {
		t.Fatalf("GetSwitch(1) failed: %v", err)
	}
	if sw == nil {
		t.Fatal("GetSwitch(1) returned nil switch")
	}

	// Test invalid switch ID
	_, err = dsc.GetSwitch(switchCount)
	if err == nil {
		t.Error("GetSwitch() with invalid ID should return error")
	}

	// Test initial state (all switches should be off)
	state, err := dsc.GetState()
	if err != nil {
		t.Fatalf("GetState() failed: %v", err)
	}
	if state {
		t.Error("Initial state should be false (all switches off)")
	}

	// Test GetDetailedState
	states, err := dsc.GetDetailedState()
	if err != nil {
		t.Fatalf("GetDetailedState() failed: %v", err)
	}
	if len(states) != int(switchCount) {
		t.Errorf("GetDetailedState() returned %d states, want %d", len(states), switchCount)
	}
	for i, s := range states {
		if s {
			t.Errorf("Switch %d should be initially off", i)
		}
	}

	// Test turning on individual switch
	if err := sw.TurnOn(); err != nil {
		t.Fatalf("TurnOn() failed: %v", err)
	}

	swState, err := sw.GetState()
	if err != nil {
		t.Fatalf("GetState() failed: %v", err)
	}
	if !swState {
		t.Error("Switch should be on after TurnOn()")
	}

	// Test that collection state is still false (not all switches on)
	state, err = dsc.GetState()
	if err != nil {
		t.Fatalf("GetState() failed: %v", err)
	}
	if state {
		t.Error("Collection state should be false when not all switches are on")
	}

	// Test turning on all switches
	if err := dsc.TurnOn(); err != nil {
		t.Fatalf("TurnOn() all switches failed: %v", err)
	}

	// Test collection state is now true
	state, err = dsc.GetState()
	if err != nil {
		t.Fatalf("GetState() failed: %v", err)
	}
	if !state {
		t.Error("Collection state should be true when all switches are on")
	}

	// Test detailed state
	states, err = dsc.GetDetailedState()
	if err != nil {
		t.Fatalf("GetDetailedState() failed: %v", err)
	}
	for i, s := range states {
		if !s {
			t.Errorf("Switch %d should be on", i)
		}
	}

	// Test turning off individual switch
	if err := sw.TurnOff(); err != nil {
		t.Fatalf("TurnOff() failed: %v", err)
	}

	swState, err = sw.GetState()
	if err != nil {
		t.Fatalf("GetState() failed: %v", err)
	}
	if swState {
		t.Error("Switch should be off after TurnOff()")
	}

	// Test turning off all switches
	if err := dsc.TurnOff(); err != nil {
		t.Fatalf("TurnOff() all switches failed: %v", err)
	}

	// Test Close
	if err := dsc.Close(); err != nil {
		t.Fatalf("Close() failed: %v", err)
	}
}

func TestDummySwitch(t *testing.T) {
	ds := &DummySwitch{id: 42, state: false}

	// Test initial state
	state, err := ds.GetState()
	if err != nil {
		t.Fatalf("GetState() failed: %v", err)
	}
	if state {
		t.Error("Initial state should be false")
	}

	// Test TurnOn
	if err := ds.TurnOn(); err != nil {
		t.Fatalf("TurnOn() failed: %v", err)
	}

	state, err = ds.GetState()
	if err != nil {
		t.Fatalf("GetState() failed: %v", err)
	}
	if !state {
		t.Error("State should be true after TurnOn()")
	}

	// Test TurnOff
	if err := ds.TurnOff(); err != nil {
		t.Fatalf("TurnOff() failed: %v", err)
	}

	state, err = ds.GetState()
	if err != nil {
		t.Fatalf("GetState() failed: %v", err)
	}
	if state {
		t.Error("State should be false after TurnOff()")
	}

	// Test String representation
	expected := "dummy:42"
	if str := ds.String(); str != expected {
		t.Errorf("String() = %q, want %q", str, expected)
	}
}
