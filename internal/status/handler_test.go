package status

import (
	"testing"
	"time"

	"github.com/larsks/display1306/v2/display"
	"github.com/larsks/display1306/v2/display/fakedriver"
)

func TestNewHandler(t *testing.T) {
	// Create a fake display for testing
	d, err := display.NewDisplay().WithDriver(fakedriver.NewFakeSSD1306()).Build()
	if err != nil {
		t.Fatalf("Failed to build fake display: %v", err)
	}

	handler := NewHandler(d)
	if handler == nil {
		t.Fatal("NewHandler returned nil")
	}
	if handler.display == nil {
		t.Fatal("Handler display is nil")
	}
}

func TestHandlerWithFakeDisplay(t *testing.T) {
	// Create display with fake driver and build it
	d, err := display.NewDisplay().WithDriver(fakedriver.NewFakeSSD1306()).Build()
	if err != nil {
		t.Fatalf("Failed to build display: %v", err)
	}

	handler := NewHandler(d)
	if handler == nil {
		t.Fatal("NewHandler returned nil")
	}

	// Test that display can be initialized
	err = handler.display.Init()
	if err != nil {
		t.Fatalf("Failed to initialize display: %v", err)
	}
	defer handler.display.Close()

	// Test basic display operations
	err = handler.display.ClearScreen()
	if err != nil {
		t.Fatalf("Failed to clear display: %v", err)
	}

	lines := []string{"Test", "Line 1", "Line 2"}
	err = handler.display.PrintLines(0, lines)
	if err != nil {
		t.Fatalf("Failed to print lines: %v", err)
	}

	err = handler.display.Update()
	if err != nil {
		t.Fatalf("Failed to update display: %v", err)
	}
}

func TestConfigDefaults(t *testing.T) {
	cfg := NewConfig()
	if cfg == nil {
		t.Fatal("NewConfig returned nil")
	}
	if cfg.UpdateInterval != 5*time.Second {
		t.Errorf("Expected update interval 5s, got %v", cfg.UpdateInterval)
	}
	if cfg.ServerURL == "" {
		t.Error("Expected non-empty server URL")
	}
}
