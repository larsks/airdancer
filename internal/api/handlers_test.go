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
	collections := map[string]switchcollection.SwitchCollection{
		"test-collection": switchcollection.NewDummySwitchCollection(switchCount),
	}

	// Initialize the collection
	if err := collections["test-collection"].Init(); err != nil {
		t.Fatalf("Failed to initialize test switches: %v", err)
	}

	// Create some test switches
	switches := make(map[string]*ResolvedSwitch)
	for i := uint(0); i < switchCount; i++ {
		switchName := fmt.Sprintf("switch%d", i)
		sw, err := collections["test-collection"].GetSwitch(i)
		if err != nil {
			t.Fatalf("Failed to get switch %d: %v", i, err)
		}

		switches[switchName] = &ResolvedSwitch{
			Name:       switchName,
			Collection: collections["test-collection"],
			Index:      i,
			Switch:     sw,
		}
	}

	// Use the shared constructor without production middleware and no listen address for tests
	return newServerWithCollections(collections, switches, "", false)
}

func TestSendResponse(t *testing.T) {
	server := createTestServer(t, 1)
	defer server.Close()

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			server.sendResponse(w, tt.resp, tt.httpCode)

			if w.Code != tt.httpCode {
				t.Errorf("sendResponse() status = %v, want %v", w.Code, tt.httpCode)
			}

			var response APIResponse
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Errorf("sendResponse() response not valid JSON: %v", err)
			}

			if response.Status != tt.wantStatus {
				t.Errorf("sendResponse() status = %v, want %v", response.Status, tt.wantStatus)
			}

			if response.Message != tt.wantMsg {
				t.Errorf("sendResponse() message = %v, want %v", response.Message, tt.wantMsg)
			}
		})
	}
}

func TestSwitchHandler_SimpleSwitch(t *testing.T) {
	server := createTestServer(t, 2)
	defer server.Close()

	// Test turning on a switch
	reqBody := `{"state": "on"}`
	req := httptest.NewRequest("POST", "/switch/switch0", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	// Add chi route context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("name", "switch0")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Parse request and add to context (simulate middleware)
	var switchReq switchRequest
	json.NewDecoder(strings.NewReader(reqBody)).Decode(&switchReq)
	req = req.WithContext(context.WithValue(req.Context(), switchRequestKey, switchReq))

	w := httptest.NewRecorder()
	server.switchHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("switchHandler() status = %v, want %v, body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	// Verify the switch was turned on
	resolvedSwitch := server.switches["switch0"]
	state, err := resolvedSwitch.Switch.GetState()
	if err != nil {
		t.Errorf("Failed to get switch state: %v", err)
	}
	if !state {
		t.Error("Switch should be on after turning it on")
	}
}

func TestSwitchStatusHandler_SingleSwitch(t *testing.T) {
	server := createTestServer(t, 1)
	defer server.Close()

	req := httptest.NewRequest("GET", "/switch/switch0", nil)

	// Add chi route context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("name", "switch0")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	server.switchStatusHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("switchStatusHandler() status = %v, want %v", w.Code, http.StatusOK)
	}

	var response APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("switchStatusHandler() response not valid JSON: %v", err)
	}

	if response.Status != "ok" {
		t.Errorf("switchStatusHandler() status = %v, want ok", response.Status)
	}
}

func TestSwitchStatusHandler_AllSwitches(t *testing.T) {
	server := createTestServer(t, 2)
	defer server.Close()

	req := httptest.NewRequest("GET", "/switch/all", nil)

	// Add chi route context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("name", "all")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	server.switchStatusHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("switchStatusHandler() status = %v, want %v", w.Code, http.StatusOK)
	}

	var response APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("switchStatusHandler() response not valid JSON: %v", err)
	}

	if response.Status != "ok" {
		t.Errorf("switchStatusHandler() status = %v, want ok", response.Status)
	}

	// Verify the response contains switch names in the switches map
	data, ok := response.Data.(map[string]interface{})
	if !ok {
		t.Errorf("switchStatusHandler() response data is not a map")
		return
	}

	switches, ok := data["switches"].(map[string]interface{})
	if !ok {
		t.Errorf("switchStatusHandler() switches field is not a map")
		return
	}

	// Check that we have the expected switch names
	expectedSwitches := []string{"switch0", "switch1"}
	for _, expectedSwitch := range expectedSwitches {
		if _, exists := switches[expectedSwitch]; !exists {
			t.Errorf("switchStatusHandler() missing expected switch %s in response", expectedSwitch)
		}
	}

	// Verify each switch has expected fields
	for switchName, switchData := range switches {
		switchInfo, ok := switchData.(map[string]interface{})
		if !ok {
			t.Errorf("switchStatusHandler() switch %s data is not a map", switchName)
			continue
		}

		if _, exists := switchInfo["state"]; !exists {
			t.Errorf("switchStatusHandler() switch %s missing 'state' field", switchName)
		}

		if _, exists := switchInfo["currentState"]; !exists {
			t.Errorf("switchStatusHandler() switch %s missing 'currentState' field", switchName)
		}
	}
}
