package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/larsks/airdancer/internal/switchcollection"
)

// createTestServer creates a server instance with dummy switches for testing
func createTestServer(t *testing.T, switchCount uint) *Server {
	switches := switchcollection.NewDummySwitchCollection(switchCount)
	if err := switches.Init(); err != nil {
		t.Fatalf("Failed to initialize test switches: %v", err)
	}

	// Use the shared constructor without production middleware and no listen address for tests
	return newServerWithSwitches(switches, "", false)
}

func TestSendResponse(t *testing.T) {
	server := createTestServer(t, 1)

	tests := []struct {
		name       string
		resp       APIResponse
		httpCode   int
		wantStatus string
		wantMsg    string
	}{
		{
			name:       "success response",
			resp:       APIResponse{Status: "ok"},
			httpCode:   http.StatusOK,
			wantStatus: "ok",
			wantMsg:    "",
		},
		{
			name:       "error response",
			resp:       APIResponse{Status: "error", Message: "Something went wrong"},
			httpCode:   http.StatusBadRequest,
			wantStatus: "error",
			wantMsg:    "Something went wrong",
		},
		{
			name:       "success with data",
			resp:       APIResponse{Status: "ok", Data: map[string]string{"test": "value"}},
			httpCode:   http.StatusOK,
			wantStatus: "ok",
			wantMsg:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			server.sendResponse(w, tt.resp, tt.httpCode)

			if w.Code != tt.httpCode {
				t.Errorf("sendResponse() status = %v, want %v", w.Code, tt.httpCode)
			}

			contentType := w.Header().Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("sendResponse() Content-Type = %v, want application/json", contentType)
			}

			var response APIResponse
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf("Failed to parse JSON response: %v", err)
			}

			if response.Status != tt.wantStatus {
				t.Errorf("sendResponse() response status = %v, want %v", response.Status, tt.wantStatus)
			}

			if response.Message != tt.wantMsg {
				t.Errorf("sendResponse() response message = %v, want %v", response.Message, tt.wantMsg)
			}
		})
	}
}

func TestSwitchStatusHandler(t *testing.T) {
	server := createTestServer(t, 3)

	// Turn on switch 1 for testing
	sw, _ := server.switches.GetSwitch(1)
	sw.TurnOn()

	tests := []struct {
		name          string
		switchID      string
		wantStatus    int
		wantDataType  string // "single" or "all"
		checkResponse func(t *testing.T, body []byte)
	}{
		{
			name:         "get single switch status",
			switchID:     "1",
			wantStatus:   http.StatusOK,
			wantDataType: "single",
			checkResponse: func(t *testing.T, body []byte) {
				var response APIResponse
				if err := json.Unmarshal(body, &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				if response.Status != "ok" {
					t.Errorf("Expected status ok, got %v", response.Status)
				}
				data := response.Data.(map[string]any)
				if data["currentState"] != true {
					t.Errorf("Expected switch state true, got %v", data["state"])
				}
			},
		},
		{
			name:         "get all switches status",
			switchID:     "all",
			wantStatus:   http.StatusOK,
			wantDataType: "all",
			checkResponse: func(t *testing.T, body []byte) {
				var response APIResponse
				if err := json.Unmarshal(body, &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				if response.Status != "ok" {
					t.Errorf("Expected status ok, got %v", response.Status)
				}
				data := response.Data.(map[string]any)
				if data["count"] != float64(3) {
					t.Errorf("Expected count 3, got %v", data["count"])
				}
				switches := data["switches"].([]any)
				if len(switches) != 3 {
					t.Errorf("Expected 3 states, got %d", len(switches))
				}

				states := []bool{}
				for _, x := range switches {
					sw := x.(map[string]any)
					states = append(states, sw["currentState"].(bool))
				}
				// Switch 1 should be on, others off
				if states[1] != true {
					t.Errorf("Expected switch 1 to be on")
				}
				if states[0] != false || states[2] != false {
					t.Errorf("Expected switches 0 and 2 to be off")
				}
			},
		},
		{
			name:       "invalid switch ID",
			switchID:   "invalid",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "non-existent switch",
			switchID:   "99",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/switch/"+tt.switchID, nil)
			w := httptest.NewRecorder()
			server.router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("switchStatusHandler() status = %v, want %v", w.Code, tt.wantStatus)
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.Bytes())
			}
		})
	}
}

func TestSwitchHandler(t *testing.T) {
	server := createTestServer(t, 3)

	tests := []struct {
		name         string
		switchID     string
		requestBody  string
		wantStatus   int
		setupContext func(*http.Request) *http.Request
	}{
		{
			name:        "turn on single switch",
			switchID:    "1",
			requestBody: `{"state":"on"}`,
			wantStatus:  http.StatusOK,
			setupContext: func(req *http.Request) *http.Request {
				// Add validated request to context as middleware would do
				switchReq := switchRequest{State: "on"}
				ctx := context.WithValue(req.Context(), switchRequestKey, switchReq)
				return req.WithContext(ctx)
			},
		},
		{
			name:        "turn off single switch",
			switchID:    "2",
			requestBody: `{"state":"off"}`,
			wantStatus:  http.StatusOK,
			setupContext: func(req *http.Request) *http.Request {
				switchReq := switchRequest{State: "off"}
				ctx := context.WithValue(req.Context(), switchRequestKey, switchReq)
				return req.WithContext(ctx)
			},
		},
		{
			name:        "turn on all switches",
			switchID:    "all",
			requestBody: `{"state":"on"}`,
			wantStatus:  http.StatusOK,
			setupContext: func(req *http.Request) *http.Request {
				switchReq := switchRequest{State: "on"}
				ctx := context.WithValue(req.Context(), switchRequestKey, switchReq)
				return req.WithContext(ctx)
			},
		},
		{
			name:        "turn on with duration",
			switchID:    "0",
			requestBody: `{"state":"on","duration":5}`,
			wantStatus:  http.StatusOK,
			setupContext: func(req *http.Request) *http.Request {
				duration := 5
				switchReq := switchRequest{State: "on", Duration: &duration}
				ctx := context.WithValue(req.Context(), switchRequestKey, switchReq)
				return req.WithContext(ctx)
			},
		},
		{
			name:        "toggle switch from off to on",
			switchID:    "1",
			requestBody: `{"state":"toggle"}`,
			wantStatus:  http.StatusOK,
			setupContext: func(req *http.Request) *http.Request {
				switchReq := switchRequest{State: "toggle"}
				ctx := context.WithValue(req.Context(), switchRequestKey, switchReq)
				return req.WithContext(ctx)
			},
		},
		{
			name:        "toggle switch from on to off",
			switchID:    "2",
			requestBody: `{"state":"toggle"}`,
			wantStatus:  http.StatusOK,
			setupContext: func(req *http.Request) *http.Request {
				// First turn on switch 2
				sw, _ := server.switches.GetSwitch(2)
				sw.TurnOn()
				switchReq := switchRequest{State: "toggle"}
				ctx := context.WithValue(req.Context(), switchRequestKey, switchReq)
				return req.WithContext(ctx)
			},
		},
		{
			name:        "blink switch",
			switchID:    "0",
			requestBody: `{"state":"blink","period":1.0,"dutyCycle":0.5}`,
			wantStatus:  http.StatusOK,
			setupContext: func(req *http.Request) *http.Request {
				period := 1.0
				dutyCycle := 0.5
				switchReq := switchRequest{State: "blink", Period: &period, DutyCycle: &dutyCycle}
				ctx := context.WithValue(req.Context(), switchRequestKey, switchReq)
				return req.WithContext(ctx)
			},
		},
		{
			name:        "blink switch with default duty cycle",
			switchID:    "1",
			requestBody: `{"state":"blink","period":2.0}`,
			wantStatus:  http.StatusOK,
			setupContext: func(req *http.Request) *http.Request {
				period := 2.0
				switchReq := switchRequest{State: "blink", Period: &period}
				ctx := context.WithValue(req.Context(), switchRequestKey, switchReq)
				return req.WithContext(ctx)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/switch/"+tt.switchID, strings.NewReader(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")

			// Add chi route context
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", tt.switchID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			// Setup test-specific context
			if tt.setupContext != nil {
				req = tt.setupContext(req)
			}

			w := httptest.NewRecorder()
			server.switchHandler(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("switchHandler() status = %v, want %v, body: %s", w.Code, tt.wantStatus, w.Body.String())
			}

			// Verify response is valid JSON
			var response APIResponse
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Errorf("switchHandler() returned invalid JSON: %v", err)
			}
		})
	}
}

func TestSwitchToggleBehavior(t *testing.T) {
	server := createTestServer(t, 2)

	tests := []struct {
		name          string
		switchID      uint
		initialState  bool
		expectedState bool
	}{
		{
			name:          "toggle off to on",
			switchID:      0,
			initialState:  false,
			expectedState: true,
		},
		{
			name:          "toggle on to off",
			switchID:      1,
			initialState:  true,
			expectedState: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup initial state
			sw, _ := server.switches.GetSwitch(tt.switchID)
			if tt.initialState {
				sw.TurnOn()
			} else {
				sw.TurnOff()
			}

			// Verify initial state
			state, _ := sw.GetState()
			if state != tt.initialState {
				t.Fatalf("Initial state setup failed: expected %v, got %v", tt.initialState, state)
			}

			// Create toggle request
			switchReq := switchRequest{State: switchStateToggle}
			req := httptest.NewRequest("POST", fmt.Sprintf("/switch/%d", tt.switchID), strings.NewReader(`{"state":"toggle"}`))
			req.Header.Set("Content-Type", "application/json")

			// Add chi route context
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", fmt.Sprintf("%d", tt.switchID))
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			// Add request context
			ctx := context.WithValue(req.Context(), switchRequestKey, switchReq)
			req = req.WithContext(ctx)

			// Execute toggle
			w := httptest.NewRecorder()
			server.switchHandler(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("toggle failed with status %v: %s", w.Code, w.Body.String())
			}

			// Verify final state
			finalState, _ := sw.GetState()
			if finalState != tt.expectedState {
				t.Errorf("toggle didn't work: expected %v, got %v", tt.expectedState, finalState)
			}
		})
	}
}

func TestSwitchBlinkBehavior(t *testing.T) {
	server := createTestServer(t, 2)

	tests := []struct {
		name      string
		switchID  uint
		period    float64
		dutyCycle *float64
		wantError bool
	}{
		{
			name:      "blink with custom duty cycle",
			switchID:  0,
			period:    1.0,
			dutyCycle: func() *float64 { d := 0.7; return &d }(),
			wantError: false,
		},
		{
			name:      "blink with default duty cycle",
			switchID:  1,
			period:    2.0,
			dutyCycle: nil,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create blink request
			switchReq := switchRequest{
				State:     switchStateBlink,
				Period:    &tt.period,
				DutyCycle: tt.dutyCycle,
			}

			req := httptest.NewRequest("POST", fmt.Sprintf("/switch/%d", tt.switchID), strings.NewReader(`{"state":"blink"}`))
			req.Header.Set("Content-Type", "application/json")

			// Add chi route context
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", fmt.Sprintf("%d", tt.switchID))
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			// Add request context
			ctx := context.WithValue(req.Context(), switchRequestKey, switchReq)
			req = req.WithContext(ctx)

			// Execute blink
			w := httptest.NewRecorder()
			server.switchHandler(w, req)

			if tt.wantError {
				if w.Code == http.StatusOK {
					t.Error("expected error but got success")
				}
				return
			}

			if w.Code != http.StatusOK {
				t.Errorf("blink failed with status %v: %s", w.Code, w.Body.String())
				return
			}

			// Verify blinker was created
			swid := fmt.Sprintf("dummy:%d", tt.switchID)
			server.mutex.Lock()
			blinker, exists := server.blinkers[swid]
			server.mutex.Unlock()

			if !exists {
				t.Error("blinker was not created")
				return
			}

			// Verify blinker properties
			if blinker.GetPeriod() != tt.period {
				t.Errorf("blinker period: expected %v, got %v", tt.period, blinker.GetPeriod())
			}

			expectedDutyCycle := 0.5 // default
			if tt.dutyCycle != nil {
				expectedDutyCycle = *tt.dutyCycle
			}
			if blinker.GetDutyCycle() != expectedDutyCycle {
				t.Errorf("blinker duty cycle: expected %v, got %v", expectedDutyCycle, blinker.GetDutyCycle())
			}

			if !blinker.IsRunning() {
				t.Error("blinker should be running")
			}

			// Clean up - stop the blinker
			blinker.Stop()
		})
	}
}
