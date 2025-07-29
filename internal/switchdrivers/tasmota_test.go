package switchdrivers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTasmotaFactory_ParseConfig(t *testing.T) {
	factory := &TasmotaFactory{}

	tests := []struct {
		name    string
		config  map[string]interface{}
		want    *TasmotaConfig
		wantErr bool
	}{
		{
			name: "valid config with addresses",
			config: map[string]interface{}{
				"addresses": []string{"192.168.1.100", "192.168.1.101"},
				"timeout":   10,
			},
			want: &TasmotaConfig{
				Addresses: []string{"192.168.1.100", "192.168.1.101"},
				Timeout:   10,
			},
			wantErr: false,
		},
		{
			name: "valid config with interface slice",
			config: map[string]interface{}{
				"addresses": []interface{}{"192.168.1.100", "192.168.1.101"},
			},
			want: &TasmotaConfig{
				Addresses: []string{"192.168.1.100", "192.168.1.101"},
				Timeout:   0,
			},
			wantErr: false,
		},
		{
			name: "missing addresses",
			config: map[string]interface{}{
				"timeout": 5,
			},
			wantErr: true,
		},
		{
			name: "invalid address type",
			config: map[string]interface{}{
				"addresses": []interface{}{"192.168.1.100", 123},
			},
			wantErr: true,
		},
		{
			name: "invalid addresses type",
			config: map[string]interface{}{
				"addresses": "not-an-array",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := factory.parseConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got.Addresses) != len(tt.want.Addresses) {
					t.Errorf("parseConfig() addresses length = %v, want %v", len(got.Addresses), len(tt.want.Addresses))
					return
				}
				for i, addr := range got.Addresses {
					if addr != tt.want.Addresses[i] {
						t.Errorf("parseConfig() address[%d] = %v, want %v", i, addr, tt.want.Addresses[i])
					}
				}
				if got.Timeout != tt.want.Timeout {
					t.Errorf("parseConfig() timeout = %v, want %v", got.Timeout, tt.want.Timeout)
				}
			}
		})
	}
}

func TestTasmotaFactory_ValidateConfig(t *testing.T) {
	factory := &TasmotaFactory{}

	tests := []struct {
		name    string
		config  map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid config",
			config: map[string]interface{}{
				"addresses": []string{"192.168.1.100"},
			},
			wantErr: false,
		},
		{
			name: "valid config with http prefix",
			config: map[string]interface{}{
				"addresses": []string{"http://192.168.1.100"},
			},
			wantErr: false,
		},
		{
			name: "empty addresses",
			config: map[string]interface{}{
				"addresses": []string{},
			},
			wantErr: true,
		},
		{
			name: "missing addresses",
			config: map[string]interface{}{
				"timeout": 5,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := factory.ValidateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTasmotaFactory_CreateDriver(t *testing.T) {
	factory := &TasmotaFactory{}

	tests := []struct {
		name    string
		config  map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid config",
			config: map[string]interface{}{
				"addresses": []string{"192.168.1.100"},
				"timeout":   5,
			},
			wantErr: false,
		},
		{
			name: "valid config with default timeout",
			config: map[string]interface{}{
				"addresses": []string{"192.168.1.100"},
			},
			wantErr: false,
		},
		{
			name: "invalid config",
			config: map[string]interface{}{
				"addresses": []string{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driver, err := factory.CreateDriver(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateDriver() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if driver == nil {
					t.Error("CreateDriver() returned nil driver")
				}
			}
		})
	}
}

func TestTasmotaSwitch_HTTPOperations(t *testing.T) {
	// Create mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only handle requests to /cm path
		if r.URL.Path != "/cm" {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		command := r.URL.Query().Get("cmnd")

		var response TasmotaResponse
		switch command {
		case "Power+ON", "Power ON":
			response.Power = "ON"
		case "Power+OFF", "Power OFF":
			response.Power = "OFF"
		case "Power":
			response.Power = "ON" // Default state for testing
		default:
			http.Error(w, "Unknown command: "+command, http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	sw := NewTasmotaSwitch(server.URL, 5*time.Second)

	t.Run("TurnOn", func(t *testing.T) {
		err := sw.TurnOn()
		if err != nil {
			t.Errorf("TurnOn() error = %v", err)
		}
	})

	t.Run("TurnOff", func(t *testing.T) {
		err := sw.TurnOff()
		if err != nil {
			t.Errorf("TurnOff() error = %v", err)
		}
	})

	t.Run("GetState", func(t *testing.T) {
		state, err := sw.GetState()
		if err != nil {
			t.Errorf("GetState() error = %v", err)
		}
		if !state {
			t.Error("GetState() expected true, got false")
		}
	})

	t.Run("String", func(t *testing.T) {
		str := sw.String()
		expectedPrefix := "TasmotaSwitch("
		if len(str) < len(expectedPrefix) || str[:len(expectedPrefix)] != expectedPrefix {
			t.Errorf("String() = %v, expected to start with %v", str, expectedPrefix)
		}
	})
}

func TestTasmotaSwitch_HTTPErrors(t *testing.T) {
	// Test with server that returns errors
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}))
	defer server.Close()

	sw := NewTasmotaSwitch(server.URL, 5*time.Second)

	t.Run("TurnOn error", func(t *testing.T) {
		err := sw.TurnOn()
		if err == nil {
			t.Error("TurnOn() expected error, got nil")
		}
	})

	t.Run("GetState error", func(t *testing.T) {
		state, err := sw.GetState()
		if err != nil {
			t.Errorf("GetState() expected no error, got %v", err)
		}
		if state != false {
			t.Error("GetState() expected false state for disabled switch, got true")
		}
		if !sw.IsDisabled() {
			t.Error("Switch should be marked as disabled after failed GetState()")
		}
	})
}

func TestTasmotaSwitch_InvalidJSON(t *testing.T) {
	// Test with server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	sw := NewTasmotaSwitch(server.URL, 5*time.Second)

	state, err := sw.GetState()
	if err != nil {
		t.Errorf("GetState() expected no error, got %v", err)
	}
	if state != false {
		t.Error("GetState() expected false state for disabled switch, got true")
	}
	if !sw.IsDisabled() {
		t.Error("Switch should be marked as disabled after JSON parsing error")
	}
}

func TestTasmotaSwitchCollection(t *testing.T) {
	// Create mock servers
	servers := make([]*httptest.Server, 2)
	for i := range servers {
		serverIndex := i
		servers[i] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only handle requests to /cm path
			if r.URL.Path != "/cm" {
				http.Error(w, "Not found", http.StatusNotFound)
				return
			}

			command := r.URL.Query().Get("cmnd")

			var response TasmotaResponse
			switch command {
			case "Power+ON", "Power ON":
				response.Power = "ON"
			case "Power+OFF", "Power OFF":
				response.Power = "OFF"
			case "Power":
				// First server ON, second server OFF for testing
				if serverIndex == 0 {
					response.Power = "ON"
				} else {
					response.Power = "OFF"
				}
			default:
				http.Error(w, "Unknown command: "+command, http.StatusBadRequest)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}))
	}
	defer func() {
		for _, server := range servers {
			server.Close()
		}
	}()

	addresses := []string{servers[0].URL, servers[1].URL}
	collection := NewTasmotaSwitchCollection(addresses, 5*time.Second)

	t.Run("CountSwitches", func(t *testing.T) {
		count := collection.CountSwitches()
		if count != 2 {
			t.Errorf("CountSwitches() = %v, want 2", count)
		}
	})

	t.Run("ListSwitches", func(t *testing.T) {
		switches := collection.ListSwitches()
		if len(switches) != 2 {
			t.Errorf("ListSwitches() length = %v, want 2", len(switches))
		}
	})

	t.Run("GetSwitch valid", func(t *testing.T) {
		sw, err := collection.GetSwitch(0)
		if err != nil {
			t.Errorf("GetSwitch(0) error = %v", err)
		}
		if sw == nil {
			t.Error("GetSwitch(0) returned nil switch")
		}
	})

	t.Run("GetSwitch invalid", func(t *testing.T) {
		_, err := collection.GetSwitch(5)
		if err == nil {
			t.Error("GetSwitch(5) expected error, got nil")
		}
	})

	t.Run("GetState", func(t *testing.T) {
		// Should return true if any switch is on (first server returns ON)
		state, err := collection.GetState()
		if err != nil {
			t.Errorf("GetState() error = %v", err)
		}
		if !state {
			t.Error("GetState() expected true, got false")
		}
	})

	t.Run("GetDetailedState", func(t *testing.T) {
		states, err := collection.GetDetailedState()
		if err != nil {
			t.Errorf("GetDetailedState() error = %v", err)
		}
		if len(states) != 2 {
			t.Errorf("GetDetailedState() length = %v, want 2", len(states))
		}
		// First switch should be ON, second OFF
		if !states[0] {
			t.Error("GetDetailedState() first switch expected true, got false")
		}
		if states[1] {
			t.Error("GetDetailedState() second switch expected false, got true")
		}
	})

	t.Run("TurnOn", func(t *testing.T) {
		err := collection.TurnOn()
		if err != nil {
			t.Errorf("TurnOn() error = %v", err)
		}
	})

	t.Run("TurnOff", func(t *testing.T) {
		err := collection.TurnOff()
		if err != nil {
			t.Errorf("TurnOff() error = %v", err)
		}
	})

	t.Run("Init", func(t *testing.T) {
		err := collection.Init()
		if err != nil {
			t.Errorf("Init() error = %v", err)
		}
	})

	t.Run("Close", func(t *testing.T) {
		err := collection.Close()
		if err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	t.Run("String", func(t *testing.T) {
		str := collection.String()
		expected := "TasmotaSwitchCollection(2 switches)"
		if str != expected {
			t.Errorf("String() = %v, want %v", str, expected)
		}
	})
}

func TestTasmotaDriver_Registration(t *testing.T) {
	// Verify that the tasmota driver is registered
	drivers := ListDrivers()
	found := false
	for _, driver := range drivers {
		if driver == "tasmota" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Tasmota driver not found in registry")
	}
}

func TestTasmotaDriver_Integration(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only handle requests to /cm path
		if r.URL.Path != "/cm" {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		response := TasmotaResponse{Power: "ON"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := map[string]interface{}{
		"addresses": []string{server.URL},
		"timeout":   5,
	}

	// Test driver creation through registry
	driver, err := Create("tasmota", config)
	if err != nil {
		t.Fatalf("Failed to create tasmota switch driver: %v", err)
	}

	if driver == nil {
		t.Error("Tasmota switch driver should not be nil")
	}

	// Test basic functionality
	if driver.CountSwitches() != 1 {
		t.Errorf("Expected 1 switch, got %d", driver.CountSwitches())
	}
}

func TestNewTasmotaSwitch_URLHandling(t *testing.T) {
	tests := []struct {
		name     string
		address  string
		expected string
	}{
		{
			name:     "address without prefix",
			address:  "192.168.1.100",
			expected: "http://192.168.1.100",
		},
		{
			name:     "address with http prefix",
			address:  "http://192.168.1.100",
			expected: "http://192.168.1.100",
		},
		{
			name:     "address with https prefix",
			address:  "https://192.168.1.100",
			expected: "https://192.168.1.100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sw := NewTasmotaSwitch(tt.address, 5*time.Second)
			if sw.address != tt.expected {
				t.Errorf("NewTasmotaSwitch() address = %v, want %v", sw.address, tt.expected)
			}
		})
	}
}
