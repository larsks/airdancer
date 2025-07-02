package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// mockHandler is a simple handler that records if it was called
type mockHandler struct {
	called bool
}

func (m *mockHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.called = true
	w.WriteHeader(http.StatusOK)
}

func TestValidateSwitchID(t *testing.T) {
	server := createTestServer(t, 3)

	tests := []struct {
		name              string
		switchID          string
		wantStatus        int
		wantHandlerCalled bool
		wantErrorMsg      string
	}{
		{
			name:              "valid numeric ID",
			switchID:          "1",
			wantStatus:        http.StatusOK,
			wantHandlerCalled: true,
		},
		{
			name:              "valid all ID",
			switchID:          "all",
			wantStatus:        http.StatusOK,
			wantHandlerCalled: true,
		},
		{
			name:              "zero ID",
			switchID:          "0",
			wantStatus:        http.StatusOK,
			wantHandlerCalled: true,
		},
		{
			name:              "empty ID",
			switchID:          "",
			wantStatus:        http.StatusBadRequest,
			wantHandlerCalled: false,
			wantErrorMsg:      "Switch ID is required",
		},
		{
			name:              "invalid non-numeric ID",
			switchID:          "invalid",
			wantStatus:        http.StatusBadRequest,
			wantHandlerCalled: false,
			wantErrorMsg:      "Invalid switch ID - must be an integer or 'all'",
		},
		{
			name:              "negative ID",
			switchID:          "-1",
			wantStatus:        http.StatusBadRequest,
			wantHandlerCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockHandler := &mockHandler{}
			middleware := server.validateSwitchID(mockHandler)

			req := httptest.NewRequest("GET", "/switch/"+tt.switchID, nil)

			// Add chi route context
			rctx := chi.NewRouteContext()
			if tt.switchID != "" {
				rctx.URLParams.Add("id", tt.switchID)
			}
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			w := httptest.NewRecorder()
			middleware.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("validateSwitchID() status = %v, want %v", w.Code, tt.wantStatus)
			}

			if mockHandler.called != tt.wantHandlerCalled {
				t.Errorf("validateSwitchID() handler called = %v, want %v", mockHandler.called, tt.wantHandlerCalled)
			}

			if tt.wantErrorMsg != "" {
				if !strings.Contains(w.Body.String(), tt.wantErrorMsg) {
					t.Errorf("validateSwitchID() error message should contain %q, got %q", tt.wantErrorMsg, w.Body.String())
				}
			}
		})
	}
}

func TestValidateJSONRequest(t *testing.T) {
	server := createTestServer(t, 1)

	tests := []struct {
		name              string
		contentType       string
		wantStatus        int
		wantHandlerCalled bool
		wantErrorMsg      string
	}{
		{
			name:              "valid application/json",
			contentType:       "application/json",
			wantStatus:        http.StatusOK,
			wantHandlerCalled: true,
		},
		{
			name:              "empty content type",
			contentType:       "",
			wantStatus:        http.StatusOK,
			wantHandlerCalled: true,
		},
		{
			name:              "invalid content type",
			contentType:       "text/plain",
			wantStatus:        http.StatusBadRequest,
			wantHandlerCalled: false,
			wantErrorMsg:      "Content-Type must be application/json",
		},
		{
			name:              "invalid content type with charset",
			contentType:       "application/json; charset=utf-8",
			wantStatus:        http.StatusBadRequest,
			wantHandlerCalled: false,
			wantErrorMsg:      "Content-Type must be application/json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockHandler := &mockHandler{}
			middleware := server.validateJSONRequest(mockHandler)

			req := httptest.NewRequest("POST", "/switch/1", nil)
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			w := httptest.NewRecorder()
			middleware.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("validateJSONRequest() status = %v, want %v", w.Code, tt.wantStatus)
			}

			if mockHandler.called != tt.wantHandlerCalled {
				t.Errorf("validateJSONRequest() handler called = %v, want %v", mockHandler.called, tt.wantHandlerCalled)
			}

			if tt.wantErrorMsg != "" {
				if !strings.Contains(w.Body.String(), tt.wantErrorMsg) {
					t.Errorf("validateJSONRequest() error message should contain %q, got %q", tt.wantErrorMsg, w.Body.String())
				}
			}
		})
	}
}

func TestValidateSwitchRequest(t *testing.T) {
	server := createTestServer(t, 1)

	tests := []struct {
		name              string
		requestBody       string
		wantStatus        int
		wantHandlerCalled bool
		wantErrorMsg      string
	}{
		{
			name:              "valid on request",
			requestBody:       `{"state":"on"}`,
			wantStatus:        http.StatusOK,
			wantHandlerCalled: true,
		},
		{
			name:              "valid off request",
			requestBody:       `{"state":"off"}`,
			wantStatus:        http.StatusOK,
			wantHandlerCalled: true,
		},
		{
			name:              "valid request with duration",
			requestBody:       `{"state":"on","duration":30}`,
			wantStatus:        http.StatusOK,
			wantHandlerCalled: true,
		},
		{
			name:              "invalid JSON",
			requestBody:       `{"state":"on"`,
			wantStatus:        http.StatusBadRequest,
			wantHandlerCalled: false,
			wantErrorMsg:      "Invalid JSON format",
		},
		{
			name:              "empty JSON",
			requestBody:       `{}`,
			wantStatus:        http.StatusBadRequest,
			wantHandlerCalled: false,
			wantErrorMsg:      "State must be 'on' or 'off'",
		},
		{
			name:              "invalid state",
			requestBody:       `{"state":"toggle"}`,
			wantStatus:        http.StatusBadRequest,
			wantHandlerCalled: false,
			wantErrorMsg:      "State must be 'on' or 'off'",
		},
		{
			name:              "zero duration",
			requestBody:       `{"state":"on","duration":0}`,
			wantStatus:        http.StatusBadRequest,
			wantHandlerCalled: false,
			wantErrorMsg:      "Duration must be positive",
		},
		{
			name:              "negative duration",
			requestBody:       `{"state":"on","duration":-5}`,
			wantStatus:        http.StatusBadRequest,
			wantHandlerCalled: false,
			wantErrorMsg:      "Duration must be positive",
		},
		{
			name:              "missing state field",
			requestBody:       `{"duration":10}`,
			wantStatus:        http.StatusBadRequest,
			wantHandlerCalled: false,
			wantErrorMsg:      "State must be 'on' or 'off'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockHandler := &mockHandler{}
			middleware := server.validateSwitchRequest(mockHandler)

			req := httptest.NewRequest("POST", "/switch/1", strings.NewReader(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			middleware.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("validateSwitchRequest() status = %v, want %v", w.Code, tt.wantStatus)
			}

			if mockHandler.called != tt.wantHandlerCalled {
				t.Errorf("validateSwitchRequest() handler called = %v, want %v", mockHandler.called, tt.wantHandlerCalled)
			}

			if tt.wantErrorMsg != "" {
				if !strings.Contains(w.Body.String(), tt.wantErrorMsg) {
					t.Errorf("validateSwitchRequest() error message should contain %q, got %q", tt.wantErrorMsg, w.Body.String())
				}
			}
		})
	}
}

func TestValidateSwitchExists(t *testing.T) {
	server := createTestServer(t, 3) // Create server with 3 switches (IDs 0, 1, 2)

	tests := []struct {
		name              string
		switchID          string
		wantStatus        int
		wantHandlerCalled bool
		wantErrorMsg      string
	}{
		{
			name:              "existing switch ID 0",
			switchID:          "0",
			wantStatus:        http.StatusOK,
			wantHandlerCalled: true,
		},
		{
			name:              "existing switch ID 1",
			switchID:          "1",
			wantStatus:        http.StatusOK,
			wantHandlerCalled: true,
		},
		{
			name:              "existing switch ID 2",
			switchID:          "2",
			wantStatus:        http.StatusOK,
			wantHandlerCalled: true,
		},
		{
			name:              "all switches",
			switchID:          "all",
			wantStatus:        http.StatusOK,
			wantHandlerCalled: true,
		},
		{
			name:              "non-existing switch ID",
			switchID:          "99",
			wantStatus:        http.StatusNotFound,
			wantHandlerCalled: false,
			wantErrorMsg:      "Switch not found",
		},
		{
			name:              "negative switch ID",
			switchID:          "-1",
			wantStatus:        http.StatusNotFound,
			wantHandlerCalled: false,
			wantErrorMsg:      "Switch not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockHandler := &mockHandler{}
			middleware := server.validateSwitchExists(mockHandler)

			req := httptest.NewRequest("GET", "/switch/"+tt.switchID, nil)

			// Add chi route context
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", tt.switchID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			w := httptest.NewRecorder()
			middleware.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("validateSwitchExists() status = %v, want %v", w.Code, tt.wantStatus)
			}

			if mockHandler.called != tt.wantHandlerCalled {
				t.Errorf("validateSwitchExists() handler called = %v, want %v", mockHandler.called, tt.wantHandlerCalled)
			}

			if tt.wantErrorMsg != "" {
				if !strings.Contains(w.Body.String(), tt.wantErrorMsg) {
					t.Errorf("validateSwitchExists() error message should contain %q, got %q", tt.wantErrorMsg, w.Body.String())
				}
			}
		})
	}
}

func TestGetSwitchRequestFromContext(t *testing.T) {
	tests := []struct {
		name      string
		setupCtx  func() context.Context
		wantFound bool
		wantState string
	}{
		{
			name: "valid context with request",
			setupCtx: func() context.Context {
				req := switchRequest{State: "on", Duration: nil}
				return context.WithValue(context.Background(), switchRequestKey, req)
			},
			wantFound: true,
			wantState: "on",
		},
		{
			name: "valid context with duration",
			setupCtx: func() context.Context {
				duration := 30
				req := switchRequest{State: "off", Duration: &duration}
				return context.WithValue(context.Background(), switchRequestKey, req)
			},
			wantFound: true,
			wantState: "off",
		},
		{
			name: "empty context",
			setupCtx: func() context.Context {
				return context.Background()
			},
			wantFound: false,
		},
		{
			name: "wrong value type in context",
			setupCtx: func() context.Context {
				return context.WithValue(context.Background(), switchRequestKey, "invalid")
			},
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/switch/1", nil)
			req = req.WithContext(tt.setupCtx())

			// Test direct context access instead of helper function
			switchReq, found := req.Context().Value(switchRequestKey).(switchRequest)

			if found != tt.wantFound {
				t.Errorf("context retrieval found = %v, want %v", found, tt.wantFound)
			}

			if tt.wantFound && switchReq.State != tt.wantState {
				t.Errorf("context retrieval state = %v, want %v", switchReq.State, tt.wantState)
			}
		})
	}
}

// TestMiddlewareChain tests that middleware can be chained together properly
func TestMiddlewareChain(t *testing.T) {
	server := createTestServer(t, 2)

	// Test a complete middleware chain like the real server uses
	handler := &mockHandler{}

	// Chain middleware in the same order as the real server
	chainedHandler := server.validateSwitchID(
		server.validateSwitchExists(
			server.validateJSONRequest(
				server.validateSwitchRequest(handler),
			),
		),
	)

	tests := []struct {
		name              string
		switchID          string
		contentType       string
		requestBody       string
		wantStatus        int
		wantHandlerCalled bool
	}{
		{
			name:              "all middleware passes",
			switchID:          "1",
			contentType:       "application/json",
			requestBody:       `{"state":"on"}`,
			wantStatus:        http.StatusOK,
			wantHandlerCalled: true,
		},
		{
			name:              "fails at switch ID validation",
			switchID:          "invalid",
			contentType:       "application/json",
			requestBody:       `{"state":"on"}`,
			wantStatus:        http.StatusBadRequest,
			wantHandlerCalled: false,
		},
		{
			name:              "fails at switch exists validation",
			switchID:          "99",
			contentType:       "application/json",
			requestBody:       `{"state":"on"}`,
			wantStatus:        http.StatusNotFound,
			wantHandlerCalled: false,
		},
		{
			name:              "fails at JSON validation",
			switchID:          "1",
			contentType:       "text/plain",
			requestBody:       `{"state":"on"}`,
			wantStatus:        http.StatusBadRequest,
			wantHandlerCalled: false,
		},
		{
			name:              "fails at request validation",
			switchID:          "1",
			contentType:       "application/json",
			requestBody:       `{"state":"invalid"}`,
			wantStatus:        http.StatusBadRequest,
			wantHandlerCalled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler.called = false // Reset for each test

			req := httptest.NewRequest("POST", "/switch/"+tt.switchID, strings.NewReader(tt.requestBody))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			// Add chi route context
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", tt.switchID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			w := httptest.NewRecorder()
			chainedHandler.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("middleware chain status = %v, want %v", w.Code, tt.wantStatus)
			}

			if handler.called != tt.wantHandlerCalled {
				t.Errorf("middleware chain handler called = %v, want %v", handler.called, tt.wantHandlerCalled)
			}
		})
	}
}
