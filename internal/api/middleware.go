package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type (
	contextKey string
)

const switchRequestKey contextKey = "switchRequest"

// validateSwitchOrGroup validates that the switch/group name parameter is valid and exists
func (s *Server) validateSwitchOrGroup(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switchName := chi.URLParam(r, "name")

		if switchName == "" {
			s.sendError(w, "Switch name is required", http.StatusBadRequest)
			return
		}

		if switchName != "all" {
			if _, exists := s.switches[switchName]; !exists {
				if _, exists := s.groups[switchName]; !exists {
					s.sendError(w, fmt.Sprintf("Unknown switch or group name: %s", switchName), http.StatusNotFound)
					return
				}
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
			s.sendError(w, "Content-Type must be application/json", http.StatusBadRequest)
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
			s.sendError(w, "Invalid JSON format", http.StatusBadRequest)
			return
		}

		// Validate state field
		if req.State != switchStateOn && req.State != switchStateOff && req.State != switchStateBlink && req.State != switchStateToggle && req.State != switchStateFlipflop {
			s.sendError(w, "State must be 'on', 'off', 'toggle', 'blink', or 'flipflop'", http.StatusBadRequest)
			return
		}

		// Validate duration field if present
		if req.Duration != nil && *req.Duration <= 0 {
			s.sendError(w, "Duration must be positive", http.StatusBadRequest)
			return
		}

		if req.State == "blink" || req.State == "flipflop" {
			if req.Period == nil {
				s.sendError(w, fmt.Sprintf("Period is required for %s state", req.State), http.StatusBadRequest)
				return
			}
			if *req.Period <= 0 {
				s.sendError(w, "Period must be positive", http.StatusBadRequest)
				return
			}

			if req.DutyCycle != nil && (*req.DutyCycle < 0 || *req.DutyCycle > 1) {
				s.sendError(w, "DutyCycle must be between 0 and 1", http.StatusBadRequest)
				return
			}
		}

		// Additional validation for flipflop - must be used on groups only
		if req.State == "flipflop" {
			switchName := chi.URLParam(r, "name")
			if switchName == "all" {
				s.sendError(w, "Flipflop state is not supported for 'all' switches", http.StatusBadRequest)
				return
			}
			if _, exists := s.switches[switchName]; exists {
				s.sendError(w, "Flipflop state is only supported for switch groups, not individual switches", http.StatusBadRequest)
				return
			}
		}

		// Store validated request in context for handler to use
		ctx := context.WithValue(r.Context(), switchRequestKey, req)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

