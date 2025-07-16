package api

import (
	"testing"

	"github.com/larsks/airdancer/internal/switchcollection"
)

func TestSwitchGroup(t *testing.T) {
	// Create a dummy switch collection for testing
	dummyCollection := switchcollection.NewDummySwitchCollection(4)
	if err := dummyCollection.Init(); err != nil {
		t.Fatalf("Failed to initialize dummy collection: %v", err)
	}
	defer dummyCollection.Close()

	// Create some resolved switches
	sw1, _ := dummyCollection.GetSwitch(0)
	sw2, _ := dummyCollection.GetSwitch(1)

	resolvedSw1 := &ResolvedSwitch{
		Name:       "switch1",
		Collection: dummyCollection,
		Index:      0,
		Switch:     sw1,
	}

	resolvedSw2 := &ResolvedSwitch{
		Name:       "switch2",
		Collection: dummyCollection,
		Index:      1,
		Switch:     sw2,
	}

	groupSwitches := map[string]*ResolvedSwitch{
		"switch1": resolvedSw1,
		"switch2": resolvedSw2,
	}

	// Create a switch group
	group := NewSwitchGroup("test-group", groupSwitches)

	// Test basic properties
	if group.String() != "test-group" {
		t.Errorf("Expected group name 'test-group', got %s", group.String())
	}

	if group.CountSwitches() != 2 {
		t.Errorf("Expected 2 switches in group, got %d", group.CountSwitches())
	}

	// Test initial state (should be off)
	state, err := group.GetState()
	if err != nil {
		t.Fatalf("Failed to get group state: %v", err)
	}
	if state {
		t.Error("Expected group state to be false initially")
	}

	// Test turning on the group
	if err := group.TurnOn(); err != nil {
		t.Fatalf("Failed to turn on group: %v", err)
	}

	// Check that all switches are on
	state, err = group.GetState()
	if err != nil {
		t.Fatalf("Failed to get group state after turning on: %v", err)
	}
	if !state {
		t.Error("Expected group state to be true after turning on")
	}

	// Test detailed state
	detailedState, err := group.GetDetailedState()
	if err != nil {
		t.Fatalf("Failed to get detailed state: %v", err)
	}
	if len(detailedState) != 2 {
		t.Errorf("Expected 2 states in detailed state, got %d", len(detailedState))
	}
	for i, state := range detailedState {
		if !state {
			t.Errorf("Expected switch %d to be on, but it was off", i)
		}
	}

	// Test turning off the group
	if err := group.TurnOff(); err != nil {
		t.Fatalf("Failed to turn off group: %v", err)
	}

	// Check that all switches are off
	state, err = group.GetState()
	if err != nil {
		t.Fatalf("Failed to get group state after turning off: %v", err)
	}
	if state {
		t.Error("Expected group state to be false after turning off")
	}

	// Test GetSwitch
	retrievedSwitch, err := group.GetSwitch(0)
	if err != nil {
		t.Fatalf("Failed to get switch by index: %v", err)
	}
	if retrievedSwitch == nil {
		t.Error("Expected to get a switch, got nil")
	}

	// Test GetSwitch with invalid index
	_, err = group.GetSwitch(10)
	if err == nil {
		t.Error("Expected error when getting switch with invalid index")
	}

	// Test ListSwitches
	switches := group.ListSwitches()
	if len(switches) != 2 {
		t.Errorf("Expected 2 switches from ListSwitches, got %d", len(switches))
	}

	// Test Init and Close (should be no-ops)
	if err := group.Init(); err != nil {
		t.Errorf("Init should not return error, got: %v", err)
	}

	if err := group.Close(); err != nil {
		t.Errorf("Close should not return error, got: %v", err)
	}

	// Test GetSwitches
	groupSwitchesRetrieved := group.GetSwitches()
	if len(groupSwitchesRetrieved) != 2 {
		t.Errorf("Expected 2 switches from GetSwitches, got %d", len(groupSwitchesRetrieved))
	}
	if _, exists := groupSwitchesRetrieved["switch1"]; !exists {
		t.Error("Expected to find switch1 in group switches")
	}
	if _, exists := groupSwitchesRetrieved["switch2"]; !exists {
		t.Error("Expected to find switch2 in group switches")
	}
}
