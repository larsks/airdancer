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
			name: "dummy collection with switches success",
			config: &Config{
				ListenAddress: "localhost",
				ListenPort:    8080,
				Collections: map[string]CollectionConfig{
					"test-collection": {
						Driver: "dummy",
						DriverConfig: map[string]interface{}{
							"switch_count": 4,
						},
					},
				},
				Switches: map[string]SwitchConfig{
					"switch1": {Spec: "test-collection.0"},
					"switch2": {Spec: "test-collection.1"},
				},
			},
			wantError: false,
		},
		{
			name: "empty configuration",
			config: &Config{
				ListenAddress: "localhost",
				ListenPort:    8080,
				Collections:   make(map[string]CollectionConfig),
				Switches:      make(map[string]SwitchConfig),
			},
			wantError: false,
		},
		{
			name: "invalid switch spec",
			config: &Config{
				ListenAddress: "localhost",
				ListenPort:    8080,
				Collections: map[string]CollectionConfig{
					"test-collection": {
						Driver: "dummy",
						DriverConfig: map[string]interface{}{
							"switch_count": 4,
						},
					},
				},
				Switches: map[string]SwitchConfig{
					"switch1": {Spec: "invalid-spec"},
				},
			},
			wantError:     true,
			errorContains: "invalid switch spec format",
		},
		{
			name: "switch refers to non-existent collection",
			config: &Config{
				ListenAddress: "localhost",
				ListenPort:    8080,
				Collections:   make(map[string]CollectionConfig),
				Switches: map[string]SwitchConfig{
					"switch1": {Spec: "nonexistent.0"},
				},
			},
			wantError:     true,
			errorContains: "collection nonexistent not found",
		},
		{
			name: "unknown driver",
			config: &Config{
				ListenAddress: "localhost",
				ListenPort:    8080,
				Collections: map[string]CollectionConfig{
					"test-collection": {
						Driver:       "unknown",
						DriverConfig: map[string]interface{}{},
					},
				},
				Switches: make(map[string]SwitchConfig),
			},
			wantError:     true,
			errorContains: "unknown driver: unknown",
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
					if server.collections == nil {
						t.Errorf("NewServer() server.collections is nil")
					}
					if server.switches == nil {
						t.Errorf("NewServer() server.switches is nil")
					}
					if server.timers == nil {
						t.Errorf("NewServer() server.timers is nil")
					}
					if server.router == nil {
						t.Errorf("NewServer() server.router is nil")
					}

					// Test collection and switch counts
					if len(server.collections) != len(tt.config.Collections) {
						t.Errorf("NewServer() collection count = %d, want %d", len(server.collections), len(tt.config.Collections))
					}
					if len(server.switches) != len(tt.config.Switches) {
						t.Errorf("NewServer() switch count = %d, want %d", len(server.switches), len(tt.config.Switches))
					}

					// Clean up
					server.Close()
				}
			}
		})
	}
}

func TestServerClose(t *testing.T) {
	config := &Config{
		ListenAddress: "localhost",
		ListenPort:    8080,
		Collections: map[string]CollectionConfig{
			"test-collection": {
				Driver: "dummy",
				DriverConfig: map[string]interface{}{
					"switch_count": 2,
				},
			},
		},
		Switches: map[string]SwitchConfig{
			"switch1": {Spec: "test-collection.0"},
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
