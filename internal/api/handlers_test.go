package api

import (
	"context"
	"encoding/json"
	"fmt"
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
	groups := make(map[string]*SwitchGroup)
	return newServerWithCollections(collections, switches, groups, "", false)
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

// createTestServerWithGroups creates a server instance with dummy switches and groups for testing
func createTestServerWithGroups(t *testing.T, switchCount uint) *Server {
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

	// Create test groups
	groups := make(map[string]*SwitchGroup)
	if switchCount >= 4 {
		// Create "red" group with switch0 and switch1
		redSwitches := map[string]*ResolvedSwitch{
			"switch0": switches["switch0"],
			"switch1": switches["switch1"],
		}
		groups["red"] = NewSwitchGroup("red", redSwitches)

		// Create "green" group with switch2 and switch3
		greenSwitches := map[string]*ResolvedSwitch{
			"switch2": switches["switch2"],
			"switch3": switches["switch3"],
		}
		groups["green"] = NewSwitchGroup("green", greenSwitches)
	}

	return newServerWithCollections(collections, switches, groups, "", false)
}

func TestSwitchStatusHandler_AllSwitches_WithGroups(t *testing.T) {
	server := createTestServerWithGroups(t, 4)
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

	// Verify the response contains group information
	data, ok := response.Data.(map[string]interface{})
	if !ok {
		t.Errorf("switchStatusHandler() response data is not a map")
		return
	}

	groups, ok := data["groups"].(map[string]interface{})
	if !ok {
		t.Errorf("switchStatusHandler() groups field is not a map")
		return
	}

	// Check that we have the expected groups
	expectedGroups := []string{"red", "green"}
	for _, expectedGroup := range expectedGroups {
		if _, exists := groups[expectedGroup]; !exists {
			t.Errorf("switchStatusHandler() missing expected group %s in response", expectedGroup)
		}
	}

	// Verify each group has expected fields
	for groupName, groupData := range groups {
		groupInfo, ok := groupData.(map[string]interface{})
		if !ok {
			t.Errorf("switchStatusHandler() group %s data is not a map", groupName)
			continue
		}

		if _, exists := groupInfo["switches"]; !exists {
			t.Errorf("switchStatusHandler() group %s missing 'switches' field", groupName)
		}

		if _, exists := groupInfo["summary"]; !exists {
			t.Errorf("switchStatusHandler() group %s missing 'summary' field", groupName)
		}

		if _, exists := groupInfo["state"]; !exists {
			t.Errorf("switchStatusHandler() group %s missing 'state' field", groupName)
		}

		// Verify switches array
		switches, ok := groupInfo["switches"].([]interface{})
		if !ok {
			t.Errorf("switchStatusHandler() group %s switches field is not an array", groupName)
			continue
		}

		// Check that the correct switches are in each group
		switch groupName {
		case "red":
			if len(switches) != 2 {
				t.Errorf("switchStatusHandler() group %s should have 2 switches, got %d", groupName, len(switches))
			}
			expectedSwitches := map[string]bool{"switch0": false, "switch1": false}
			for _, sw := range switches {
				if swName, ok := sw.(string); ok {
					if _, exists := expectedSwitches[swName]; exists {
						expectedSwitches[swName] = true
					}
				}
			}
			for swName, found := range expectedSwitches {
				if !found {
					t.Errorf("switchStatusHandler() group %s missing expected switch %s", groupName, swName)
				}
			}
		case "green":
			if len(switches) != 2 {
				t.Errorf("switchStatusHandler() group %s should have 2 switches, got %d", groupName, len(switches))
			}
			expectedSwitches := map[string]bool{"switch2": false, "switch3": false}
			for _, sw := range switches {
				if swName, ok := sw.(string); ok {
					if _, exists := expectedSwitches[swName]; exists {
						expectedSwitches[swName] = true
					}
				}
			}
			for swName, found := range expectedSwitches {
				if !found {
					t.Errorf("switchStatusHandler() group %s missing expected switch %s", groupName, swName)
				}
			}
		}
	}
}

func TestSwitchHandler_BlinkWithTimerExpiration(t *testing.T) {
	server := createTestServer(t, 1)
	defer server.Close()

	// Test starting a blink operation with a short duration
	reqBody := `{"state": "blink", "duration": 1, "period": 0.1}`
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

	// Verify the task was created and is running
	server.mutex.Lock()
	task, exists := server.taskManager.GetTask("switch0")
	if !exists {
		server.mutex.Unlock()
		t.Fatal("Task should exist for switch0 after starting blink operation")
	}
	if !task.IsRunning() {
		server.mutex.Unlock()
		t.Fatal("Task should be running after starting blink operation")
	}
	if task.Type() != TaskTypeBlink {
		server.mutex.Unlock()
		t.Fatal("Task should be a blink task for switch0")
	}

	// Verify timer was created
	_, timerExists := server.timers["switch0"]
	if !timerExists {
		server.mutex.Unlock()
		t.Fatal("Timer should exist for switch0 after starting timed blink operation")
	}
	server.mutex.Unlock()

	// Wait for timer to expire (1 second + small buffer)
	time.Sleep(1200 * time.Millisecond)

	// Verify blinker was stopped and cleaned up after timer expiration
	server.mutex.Lock()
	defer server.mutex.Unlock()

	// Check that task is cleaned up
	if _, exists := server.taskManager.GetTask("switch0"); exists {
		t.Error("Task should be cleaned up after timer expiration")
	}

	// Check that timer is cleaned up
	if _, exists := server.timers["switch0"]; exists {
		t.Error("Timer should be cleaned up after expiration")
	}

	// Verify the switch was turned off
	resolvedSwitch := server.switches["switch0"]
	state, err := resolvedSwitch.Switch.GetState()
	if err != nil {
		t.Errorf("Failed to get switch state: %v", err)
	}
	if state {
		t.Error("Switch should be off after timer expiration")
	}

	// Additional verification: task should no longer exist
	// The task cleanup is already verified above, so no additional check needed
}
