package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// contextKey is a custom type to avoid context key collisions
type contextKey string

const (
	switchRequestKey contextKey = "switchRequest"
)

// validateSwitchID validates that the switch ID parameter is either "all" or a valid integer
func (s *Server) validateSwitchID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switchIDStr := chi.URLParam(r, "id")

		if switchIDStr == "" {
			s.sendJSONResponse(w, "error", "Switch ID is required", http.StatusBadRequest)
			return
		}

		if switchIDStr != "all" {
			if _, err := strconv.Atoi(switchIDStr); err != nil {
				s.sendJSONResponse(w, "error", "Invalid switch ID - must be an integer or 'all'", http.StatusBadRequest)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// validateJSONRequest validates that the request has proper JSON content type
func (s *Server) validateJSONRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType := r.Header.Get("Content-Type")
		if contentType != "" && contentType != "application/json" {
			s.sendJSONResponse(w, "error", "Content-Type must be application/json", http.StatusBadRequest)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// validateSwitchRequest validates and parses the switch request JSON body
func (s *Server) validateSwitchRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req switchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.sendJSONResponse(w, "error", "Invalid JSON format", http.StatusBadRequest)
			return
		}

		// Validate state field
		if req.State != "on" && req.State != "off" {
			s.sendJSONResponse(w, "error", "State must be 'on' or 'off'", http.StatusBadRequest)
			return
		}

		// Validate duration field if present
		if req.Duration != nil && *req.Duration <= 0 {
			s.sendJSONResponse(w, "error", "Duration must be positive", http.StatusBadRequest)
			return
		}

		// Store validated request in context for handler to use
		ctx := context.WithValue(r.Context(), switchRequestKey, req)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// validateSwitchExists validates that the requested switch(es) exist
func (s *Server) validateSwitchExists(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switchIDStr := chi.URLParam(r, "id")

		if switchIDStr != "all" {
			id, _ := strconv.Atoi(switchIDStr) // Already validated by validateSwitchID
			if _, err := s.switches.GetSwitch(uint(id)); err != nil {
				s.sendJSONResponse(w, "error", "Switch not found", http.StatusNotFound)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// getSwitchRequestFromContext retrieves the validated switch request from context
func getSwitchRequestFromContext(r *http.Request) (switchRequest, bool) {
	req, ok := r.Context().Value(switchRequestKey).(switchRequest)
	return req, ok
}
