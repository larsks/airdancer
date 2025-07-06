package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

type (
	contextKey string
)

const switchRequestKey contextKey = "switchRequest"

// validateSwitchID validates that the switch ID parameter is either "all" or a valid switch id
func (s *Server) validateSwitchID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switchIDStr := chi.URLParam(r, "id")

		if switchIDStr == "" {
			s.sendError(w, "Switch ID is required", http.StatusBadRequest)
			return
		}

		if switchIDStr != "all" {
			val, err := strconv.Atoi(switchIDStr)
			if err != nil {
				s.sendError(w, "Invalid switch ID - must be an integer or 'all'", http.StatusBadRequest)
				return
			}

			if val < 0 {
				s.sendError(w, "Invalid switch ID -- must be >= 0", http.StatusBadRequest)
				return
			}

			if val >= int(s.switches.CountSwitches()) {
				s.sendError(w, fmt.Sprintf("Invalid switch ID -- must be < %d", s.switches.CountSwitches()), http.StatusBadRequest)
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
		if req.State != switchStateOn && req.State != switchStateOff && req.State != switchStateBlink {
			s.sendError(w, "State must be 'on', 'off', or 'blink'", http.StatusBadRequest)
			return
		}

		// Validate duration field if present
		if req.Duration != nil && *req.Duration <= 0 {
			s.sendError(w, "Duration must be positive", http.StatusBadRequest)
			return
		}

		if req.State == "blink" {
			if req.Period == nil {
				s.sendError(w, "Period is required for blink state", http.StatusBadRequest)
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
				s.sendError(w, "Switch not found", http.StatusNotFound)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
