package api

import (
	"strings"
	"testing"
)

func TestNewServer(t *testing.T) {
	tests := []struct {
		name          string
		config        *Config
		wantError     bool
		errorContains string
	}{
		{
			name: "dummy driver success",
			config: &Config{
				ListenAddress: "localhost",
				ListenPort:    8080,
				Driver:        "dummy",
				DummyConfig: DummyConfig{
					SwitchCount: 4,
				},
			},
			wantError: false,
		},
		{
			name: "dummy driver with zero switches",
			config: &Config{
				ListenAddress: "localhost",
				ListenPort:    8080,
				Driver:        "dummy",
				DummyConfig: DummyConfig{
					SwitchCount: 0,
				},
			},
			wantError: false,
		},
		{
			name: "dummy driver with many switches",
			config: &Config{
				ListenAddress: "",
				ListenPort:    9090,
				Driver:        "dummy",
				DummyConfig: DummyConfig{
					SwitchCount: 100,
				},
			},
			wantError: false,
		},
		{
			name: "gpio driver with valid pins",
			config: &Config{
				ListenAddress: "localhost",
				ListenPort:    8080,
				Driver:        "gpio",
				GPIOConfig: GPIOConfig{
					Pins: []string{"GPIO18", "GPIO19"},
				},
			},
			wantError:     true, // Will fail in test environment without GPIO
			errorContains: "failed to create gpio driver",
		},
		{
			name: "gpio driver with no pins",
			config: &Config{
				ListenAddress: "localhost",
				ListenPort:    8080,
				Driver:        "gpio",
				GPIOConfig: GPIOConfig{
					Pins: []string{},
				},
			},
			wantError: false, // Empty GPIO collection is valid
		},
		{
			name: "piface driver",
			config: &Config{
				ListenAddress: "localhost",
				ListenPort:    8080,
				Driver:        "piface",
				PiFaceConfig: PiFaceConfig{
					SPIDev: "/dev/spidev0.0",
				},
			},
			wantError:     true, // Will fail in test environment without PiFace hardware
			errorContains: "failed to open PiFace",
		},
		{
			name: "unknown driver",
			config: &Config{
				ListenAddress: "localhost",
				ListenPort:    8080,
				Driver:        "unknown",
			},
			wantError:     true,
			errorContains: "unknown driver: unknown",
		},
		{
			name: "empty driver string",
			config: &Config{
				ListenAddress: "localhost",
				ListenPort:    8080,
				Driver:        "",
			},
			wantError:     true,
			errorContains: "unknown driver:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := NewServer(tt.config)

			if tt.wantError {
				if err == nil {
					t.Errorf("NewServer() expected error but got none")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("NewServer() error = %v, want to contain %q", err, tt.errorContains)
				}
				if server != nil {
					t.Errorf("NewServer() expected nil server on error, got %v", server)
				}
			} else {
				if err != nil {
					t.Errorf("NewServer() unexpected error = %v", err)
				}
				if server == nil {
					t.Errorf("NewServer() expected server but got nil")
				} else {
					// Test that server has expected properties
					if server.switches == nil {
						t.Errorf("NewServer() server.switches is nil")
					}
					if server.timers == nil {
						t.Errorf("NewServer() server.timers is nil")
					}
					if server.router == nil {
						t.Errorf("NewServer() server.router is nil")
					}

					// Test switch count for dummy driver
					if tt.config.Driver == "dummy" {
						count := server.switches.CountSwitches()
						if count != tt.config.DummyConfig.SwitchCount {
							t.Errorf("NewServer() switch count = %d, want %d", count, tt.config.DummyConfig.SwitchCount)
						}
					}

					// Clean up
					server.Close()
				}
			}
		})
	}
}

func TestServerInitialization(t *testing.T) {
	// Test that the server properly initializes switches to off state
	config := &Config{
		ListenAddress: "localhost",
		ListenPort:    8080,
		Driver:        "dummy",
		DummyConfig: DummyConfig{
			SwitchCount: 3,
		},
	}

	server, err := NewServer(config)
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}
	defer server.Close()

	// Check that all switches are initially off
	states, err := server.switches.GetDetailedState()
	if err != nil {
		t.Fatalf("GetDetailedState() failed: %v", err)
	}

	for i, state := range states {
		if state {
			t.Errorf("Switch %d should be initially off, but was on", i)
		}
	}

	// Check that summary state is false (not all switches on)
	summaryState, err := server.switches.GetState()
	if err != nil {
		t.Fatalf("GetState() failed: %v", err)
	}

	if summaryState {
		t.Errorf("Summary state should be false when all switches are off")
	}
}

func TestServerClose(t *testing.T) {
	config := &Config{
		ListenAddress: "localhost",
		ListenPort:    8080,
		Driver:        "dummy",
		DummyConfig: DummyConfig{
			SwitchCount: 2,
		},
	}

	server, err := NewServer(config)
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}

	// Test that Close() doesn't return error
	err = server.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}

	// Test that we can call Close() multiple times without error
	err = server.Close()
	if err != nil {
		t.Errorf("Second Close() returned error: %v", err)
	}
}
