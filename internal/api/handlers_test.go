package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/larsks/airdancer/internal/switchcollection"
)

// createTestServer creates a server instance with dummy switches for testing
func createTestServer(t *testing.T, switchCount uint) *Server {
	switches := switchcollection.NewDummySwitchCollection(switchCount)
	if err := switches.Init(); err != nil {
		t.Fatalf("Failed to initialize test switches: %v", err)
	}

	server := &Server{
		switches: switches,
		timers:   make(map[string]*time.Timer),
		router:   chi.NewRouter(),
	}

	return server
}

func TestSendJSONResponse(t *testing.T) {
	server := createTestServer(t, 1)

	tests := []struct {
		name       string
		status     string
		message    string
		httpCode   int
		wantStatus string
		wantMsg    string
	}{
		{
			name:       "success response",
			status:     "ok",
			message:    "",
			httpCode:   http.StatusOK,
			wantStatus: "ok",
			wantMsg:    "",
		},
		{
			name:       "error response",
			status:     "error",
			message:    "Something went wrong",
			httpCode:   http.StatusBadRequest,
			wantStatus: "error",
			wantMsg:    "Something went wrong",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			server.sendJSONResponse(w, tt.status, tt.message, tt.httpCode)

			if w.Code != tt.httpCode {
				t.Errorf("sendJSONResponse() status = %v, want %v", w.Code, tt.httpCode)
			}

			contentType := w.Header().Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("sendJSONResponse() Content-Type = %v, want application/json", contentType)
			}

			var response jsonResponse
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf("Failed to parse JSON response: %v", err)
			}

			if response.Status != tt.wantStatus {
				t.Errorf("sendJSONResponse() response status = %v, want %v", response.Status, tt.wantStatus)
			}

			if response.Message != tt.wantMsg {
				t.Errorf("sendJSONResponse() response message = %v, want %v", response.Message, tt.wantMsg)
			}
		})
	}
}

func TestGetSwitchesFromRequest(t *testing.T) {
	server := createTestServer(t, 3)

	tests := []struct {
		name          string
		switchID      string
		wantCount     int
		wantError     bool
		errorContains string
	}{
		{
			name:      "get all switches",
			switchID:  "all",
			wantCount: 1, // Returns the collection itself
			wantError: false,
		},
		{
			name:      "get valid switch",
			switchID:  "1",
			wantCount: 1,
			wantError: false,
		},
		{
			name:          "get invalid switch ID",
			switchID:      "99",
			wantCount:     0,
			wantError:     true,
			errorContains: "invalid switch id",
		},
		{
			name:          "get non-numeric switch ID",
			switchID:      "invalid",
			wantCount:     0,
			wantError:     true,
			errorContains: "invalid syntax",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a request with the switch ID parameter
			req := httptest.NewRequest("GET", "/switch/"+tt.switchID, nil)

			// Create a chi context with the URL parameter
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", tt.switchID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			switches, err := server.getSwitchesFromRequest(req)

			if tt.wantError {
				if err == nil {
					t.Errorf("getSwitchesFromRequest() expected error but got none")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("getSwitchesFromRequest() error = %v, want to contain %v", err, tt.errorContains)
				}
			} else {
				if err != nil {
					t.Errorf("getSwitchesFromRequest() unexpected error = %v", err)
				}
				if len(switches) != tt.wantCount {
					t.Errorf("getSwitchesFromRequest() returned %d switches, want %d", len(switches), tt.wantCount)
				}
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
				var response switchStatusResponse
				if err := json.Unmarshal(body, &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				data := response.Data.(map[string]interface{})
				if data["id"] != "1" {
					t.Errorf("Expected switch ID 1, got %v", data["id"])
				}
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
				var response switchStatusResponse
				if err := json.Unmarshal(body, &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/switch/"+tt.switchID, nil)

			// Add chi route context
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", tt.switchID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			w := httptest.NewRecorder()
			server.switchStatusHandler(w, req)

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
			var response jsonResponse
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Errorf("switchHandler() returned invalid JSON: %v", err)
			}
		})
	}
}
