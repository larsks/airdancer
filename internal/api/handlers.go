package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

type switchRequest struct {
	State    string `json:"state"`
	Duration *int   `json:"duration,omitempty"`
}

// Single response type that handles all cases
type APIResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// Helper methods for responses
func (s *Server) sendSuccess(w http.ResponseWriter, data interface{}) {
	s.sendResponse(w, APIResponse{Status: "ok", Data: data}, http.StatusOK)
}

func (s *Server) sendError(w http.ResponseWriter, message string, code int) {
	s.sendResponse(w, APIResponse{Status: "error", Message: message}, code)
}

func (s *Server) sendResponse(w http.ResponseWriter, resp APIResponse, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(resp) //nolint:errcheck
}

func (s *Server) switchHandler(w http.ResponseWriter, r *http.Request) {
	switchID := chi.URLParam(r, "id")

	// Verify validated request exists in context
	if _, ok := r.Context().Value(switchRequestKey).(switchRequest); !ok {
		s.sendError(w, "Internal error: missing request data", http.StatusInternalServerError)
		return
	}

	if switchID == "all" {
		s.handleAllSwitches(w, r)
	} else {
		id, err := strconv.Atoi(switchID)
		if err != nil {
			s.sendError(w, "Invalid switch ID - must be an integer or 'all'", http.StatusBadRequest)
			return
		}
		s.handleSingleSwitch(w, r, uint(id))
	}
}

func (s *Server) handleAllSwitches(w http.ResponseWriter, r *http.Request) {
	// Get validated request directly from context
	req, ok := r.Context().Value(switchRequestKey).(switchRequest)
	if !ok {
		s.sendError(w, "Internal error: missing request data", http.StatusInternalServerError)
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock() //nolint:errcheck

	// Cancel any existing timers for all switches
	for swid, timer := range s.timers {
		log.Printf("Cancelling timer on %s", swid)
		timer.Stop()
		delete(s.timers, swid)
	}

	// Execute switch operation on all switches
	switch req.State {
	case "on":
		if err := s.switches.TurnOn(); err != nil {
			s.sendError(w, "Failed to turn on switches", http.StatusInternalServerError)
			return
		}
		// Set up auto-off timer if duration specified
		if req.Duration != nil {
			duration := time.Duration(*req.Duration) * time.Second
			swid := s.switches.String()
			s.timers[swid] = time.AfterFunc(duration, func() {
				s.mutex.Lock()
				defer s.mutex.Unlock() //nolint:errcheck
				delete(s.timers, swid)
				if err := s.switches.TurnOff(); err != nil {
					log.Printf("Failed to automatically turn off all switches: %v", err)
				}
				log.Printf("Automatically turned off all switches after %s", duration)
			})
		}
	case "off":
		if err := s.switches.TurnOff(); err != nil {
			s.sendError(w, "Failed to turn off switches", http.StatusInternalServerError)
			return
		}
	}

	s.sendSuccess(w, nil)
}

func (s *Server) handleSingleSwitch(w http.ResponseWriter, r *http.Request, id uint) {
	// Get validated request directly from context
	req, ok := r.Context().Value(switchRequestKey).(switchRequest)
	if !ok {
		s.sendError(w, "Internal error: missing request data", http.StatusInternalServerError)
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock() //nolint:errcheck

	// Check if switch exists
	sw, err := s.switches.GetSwitch(id)
	if err != nil {
		s.sendError(w, "Switch not found", http.StatusNotFound)
		return
	}

	swid := sw.String()

	// Cancel any existing timer for this switch
	if timer, ok := s.timers[swid]; ok {
		log.Printf("Cancelling timer on %s", swid)
		timer.Stop()
		delete(s.timers, swid)
	}

	// Execute switch operation
	switch req.State {
	case "on":
		if err := sw.TurnOn(); err != nil {
			s.sendError(w, "Failed to turn on switch", http.StatusInternalServerError)
			return
		}
		// Set up auto-off timer if duration specified
		if req.Duration != nil {
			duration := time.Duration(*req.Duration) * time.Second
			s.timers[swid] = time.AfterFunc(duration, func() {
				s.mutex.Lock()
				defer s.mutex.Unlock() //nolint:errcheck
				delete(s.timers, swid)
				if err := sw.TurnOff(); err != nil {
					log.Printf("Failed to automatically turn off switch %s: %v", swid, err)
				}
				log.Printf("Automatically turned off switch %s after %s", swid, duration)
			})
		}
	case "off":
		if err := sw.TurnOff(); err != nil {
			s.sendError(w, "Failed to turn off switch", http.StatusInternalServerError)
			return
		}
	}

	s.sendSuccess(w, nil)
}

func (s *Server) switchStatusHandler(w http.ResponseWriter, r *http.Request) {
	switchIDStr := chi.URLParam(r, "id")

	// Validate switch ID
	if switchIDStr == "" {
		s.sendError(w, "Switch ID is required", http.StatusBadRequest)
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock() //nolint:errcheck

	if switchIDStr == "all" {
		s.handleAllSwitchesStatus(w)
	} else {
		id, err := strconv.Atoi(switchIDStr)
		if err != nil {
			s.sendError(w, "Invalid switch ID - must be an integer or 'all'", http.StatusBadRequest)
			return
		}
		s.handleSingleSwitchStatus(w, uint(id), switchIDStr)
	}
}

func (s *Server) handleAllSwitchesStatus(w http.ResponseWriter) {
	// Get detailed state for all switches
	states, err := s.switches.GetDetailedState()
	if err != nil {
		log.Printf("Failed to get detailed switch states: %v", err)
		s.sendError(w, "Failed to get switch states", http.StatusInternalServerError)
		return
	}

	// Get summary state (true if all switches are on)
	summary, err := s.switches.GetState()
	if err != nil {
		log.Printf("Failed to get summary switch state: %v", err)
		s.sendError(w, "Failed to get switch state", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"count":   s.switches.CountSwitches(),
		"states":  states,
		"summary": summary,
	}

	s.sendSuccess(w, data)
}

func (s *Server) handleSingleSwitchStatus(w http.ResponseWriter, id uint, idStr string) {
	// Check if switch exists
	sw, err := s.switches.GetSwitch(id)
	if err != nil {
		s.sendError(w, "Switch not found", http.StatusNotFound)
		return
	}

	state, err := sw.GetState()
	if err != nil {
		log.Printf("Failed to get state for switch %d: %v", id, err)
		s.sendError(w, "Failed to get switch state", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"id":    idStr,
		"state": state,
	}

	s.sendSuccess(w, data)
}
