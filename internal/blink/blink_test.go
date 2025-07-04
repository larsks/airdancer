package blink

import (
	"testing"
	"time"

	"github.com/larsks/airdancer/internal/switchcollection"
)

func TestNewBlink(t *testing.T) {
	// Test creating a blink with valid parameters
	sw := &switchcollection.DummySwitch{}
	period := 0.5 // 0.5 seconds, equivalent to 2Hz

	blink, err := NewBlink(sw, period)
	if err != nil {
		t.Fatalf("NewBlink() failed: %v", err)
	}

	if blink.GetPeriod() != period {
		t.Errorf("GetPeriod() = %f, want %f", blink.GetPeriod(), period)
	}

	if blink.GetSwitch() != sw {
		t.Error("GetSwitch() returned different switch than expected")
	}

	if blink.IsRunning() {
		t.Error("Blink should not be running initially")
	}
}

func TestNewBlinkErrors(t *testing.T) {
	tests := []struct {
		name    string
		sw      switchcollection.Switch
		period  float64
		wantErr error
	}{
		{
			name:    "nil switch",
			sw:      nil,
			period:  1.0,
			wantErr: ErrSwitchRequired,
		},
		{
			name:    "zero period",
			sw:      &switchcollection.DummySwitch{},
			period:  0.0,
			wantErr: ErrInvalidPeriod,
		},
		{
			name:    "negative period",
			sw:      &switchcollection.DummySwitch{},
			period:  -1.0,
			wantErr: ErrInvalidPeriod,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewBlink(tt.sw, tt.period)
			if err != tt.wantErr {
				t.Errorf("NewBlink() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestBlinkStartStop(t *testing.T) {
	sw := &switchcollection.DummySwitch{}
	blink, err := NewBlink(sw, 0.1) // 0.1s period for faster testing
	if err != nil {
		t.Fatalf("NewBlink() failed: %v", err)
	}

	// Test starting
	if err := blink.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	if !blink.IsRunning() {
		t.Error("Blink should be running after Start()")
	}

	// Test starting again (should return error)
	if err := blink.Start(); err != ErrAlreadyRunning {
		t.Errorf("Start() while running should return ErrAlreadyRunning, got %v", err)
	}

	// Wait a bit to let the blink cycle
	time.Sleep(150 * time.Millisecond)

	// Test stopping
	if err := blink.Stop(); err != nil {
		t.Fatalf("Stop() failed: %v", err)
	}

	if blink.IsRunning() {
		t.Error("Blink should not be running after Stop()")
	}

	// Test stopping again (should return error)
	if err := blink.Stop(); err != ErrNotRunning {
		t.Errorf("Stop() while not running should return ErrNotRunning, got %v", err)
	}

	// Verify switch is off after stop
	state, err := sw.GetState()
	if err != nil {
		t.Fatalf("GetState() failed: %v", err)
	}
	if state {
		t.Error("Switch should be off after Stop()")
	}
}

func TestBlinkRestartability(t *testing.T) {
	sw := &switchcollection.DummySwitch{}
	blink, err := NewBlink(sw, 0.1)
	if err != nil {
		t.Fatalf("NewBlink() failed: %v", err)
	}

	// First cycle
	if err := blink.Start(); err != nil {
		t.Fatalf("First Start() failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if err := blink.Stop(); err != nil {
		t.Fatalf("First Stop() failed: %v", err)
	}

	// Second cycle - should be able to restart
	if err := blink.Start(); err != nil {
		t.Fatalf("Second Start() failed: %v", err)
	}

	if !blink.IsRunning() {
		t.Error("Blink should be running after restart")
	}

	if err := blink.Stop(); err != nil {
		t.Fatalf("Second Stop() failed: %v", err)
	}
}

func TestBlinkPeriod(t *testing.T) {
	sw := &switchcollection.DummySwitch{}

	// Test with 0.2s period (should toggle every 100ms)
	blink, err := NewBlink(sw, 0.2)
	if err != nil {
		t.Fatalf("NewBlink() failed: %v", err)
	}

	if err := blink.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Initial state should be off
	state, err := sw.GetState()
	if err != nil {
		t.Fatalf("GetState() failed: %v", err)
	}
	if state {
		t.Error("Switch should start in off state")
	}

	// Wait for first toggle (on)
	time.Sleep(110 * time.Millisecond)
	state, err = sw.GetState()
	if err != nil {
		t.Fatalf("GetState() failed: %v", err)
	}
	if !state {
		t.Error("Switch should be on after first toggle")
	}

	// Wait for second toggle (off)
	time.Sleep(110 * time.Millisecond)
	state, err = sw.GetState()
	if err != nil {
		t.Fatalf("GetState() failed: %v", err)
	}
	if state {
		t.Error("Switch should be off after second toggle")
	}

	if err := blink.Stop(); err != nil {
		t.Fatalf("Stop() failed: %v", err)
	}
}

func TestBlinkConcurrency(t *testing.T) {
	sw := &switchcollection.DummySwitch{}
	blink, err := NewBlink(sw, 0.05) // High frequency for more concurrent operations
	if err != nil {
		t.Fatalf("NewBlink() failed: %v", err)
	}

	// Test concurrent access to getters while running
	if err := blink.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	done := make(chan bool)

	// Goroutine accessing getters
	go func() {
		for range 100 {
			_ = blink.IsRunning()
			_ = blink.GetPeriod()
			_ = blink.GetSwitch()
			time.Sleep(time.Millisecond)
		}
		done <- true
	}()

	// Let it run for a bit
	time.Sleep(100 * time.Millisecond)

	if err := blink.Stop(); err != nil {
		t.Fatalf("Stop() failed: %v", err)
	}

	<-done // Wait for goroutine to finish
}
