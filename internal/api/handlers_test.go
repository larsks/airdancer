package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/larsks/airdancer/internal/blink"
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
				data := response.Data.(map[string]interface{})
				if data["state"] != true {
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
				data := response.Data.(map[string]interface{})
				if data["count"] != float64(3) {
					t.Errorf("Expected count 3, got %v", data["count"])
				}
				states := data["states"].([]interface{})
				if len(states) != 3 {
					t.Errorf("Expected 3 states, got %d", len(states))
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
			name:        "missing request context",
			switchID:    "1",
			requestBody: `{"state":"on"}`,
			wantStatus:  http.StatusInternalServerError,
			setupContext: func(req *http.Request) *http.Request {
				// Don't add context - simulate middleware failure
				return req
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

func TestBlinkStatusHandler(t *testing.T) {
	server := createTestServer(t, 3)

	// Set up a blinker on switch 1
	sw1, _ := server.switches.GetSwitch(1)
	blinker, err := blink.NewBlink(sw1, 0.5)
	if err != nil {
		t.Fatalf("Failed to create blinker: %v", err)
	}
	server.blinkers[sw1.String()] = blinker
	if err := blinker.Start(); err != nil {
		t.Fatalf("Failed to start blinker: %v", err)
	}

	tests := []struct {
		name          string
		switchID      string
		wantStatus    int
		checkResponse func(t *testing.T, body []byte)
	}{
		{
			name:       "switch not blinking",
			switchID:   "0",
			wantStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var response APIResponse
				if err := json.Unmarshal(body, &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				if response.Status != "ok" {
					t.Errorf("Expected status ok, got %v", response.Status)
				}
				data := response.Data.(map[string]interface{})
				if data["blinking"] != false {
					t.Errorf("Expected blinking false, got %v", data["blinking"])
				}
				if _, exists := data["period"]; exists {
					t.Error("Expected no period field when not blinking")
				}
			},
		},
		{
			name:       "switch blinking",
			switchID:   "1",
			wantStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var response APIResponse
				if err := json.Unmarshal(body, &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				if response.Status != "ok" {
					t.Errorf("Expected status ok, got %v", response.Status)
				}
				data := response.Data.(map[string]interface{})
				if data["blinking"] != true {
					t.Errorf("Expected blinking true, got %v", data["blinking"])
				}
				if data["period"] != 0.5 {
					t.Errorf("Expected period 0.5, got %v", data["period"])
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
			req := httptest.NewRequest("GET", "/switch/"+tt.switchID+"/blink", nil)
			w := httptest.NewRecorder()
			server.router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("blinkStatusHandler() status = %v, want %v", w.Code, tt.wantStatus)
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.Bytes())
			}
		})
	}

	// Clean up
	if err := blinker.Stop(); err != nil {
		t.Errorf("Failed to stop blinker: %v", err)
	}
}
