package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/larsks/airdancer/internal/switchcollection"
)

type switchRequest struct {
	State    string `json:"state"`
	Duration *int   `json:"duration,omitempty"`
}

type jsonResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type switchStatusResponse struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data"`
}

type singleSwitchStatus struct {
	ID    string `json:"id"`
	State bool   `json:"state"`
}

type allSwitchesStatus struct {
	Count   uint   `json:"count"`
	States  []bool `json:"states"`
	Summary bool   `json:"summary"`
}

func (s *Server) sendJSONResponse(w http.ResponseWriter, status string, message string, httpCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpCode)
	json.NewEncoder(w).Encode(jsonResponse{
		Status:  status,
		Message: message,
	})
}

func (s *Server) getSwitchesFromRequest(r *http.Request) ([]switchcollection.Switch, error) {
	switchIDStr := chi.URLParam(r, "id")
	var switches []switchcollection.Switch
	if switchIDStr == "all" {
		switches = append(switches, s.switches)
	} else {
		id, err := strconv.Atoi(switchIDStr)
		if err != nil {
			return nil, err
		}

		sw, err := s.switches.GetSwitch(uint(id))
		if err != nil {
			return nil, err
		}

		switches = append(switches, sw)
	}

	return switches, nil
}

func (s *Server) switchHandler(w http.ResponseWriter, r *http.Request) {
	// Get pre-validated request from context
	req, ok := getSwitchRequestFromContext(r)
	if !ok {
		s.sendJSONResponse(w, "error", "Internal error: missing request data", http.StatusInternalServerError)
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Get switches - validation already done by middleware
	switches, err := s.getSwitchesFromRequest(r)
	if err != nil {
		log.Printf("Failed to get list of switches: %v", err)
		s.sendJSONResponse(w, "error", "Failed to get list of switches", http.StatusInternalServerError)
		return
	}

	// Process each switch - validation already done by middleware
	for _, sw := range switches {
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
				s.sendJSONResponse(w, "error", "Failed to turn on switch", http.StatusInternalServerError)
				return
			}
			// Set up auto-off timer if duration specified
			if req.Duration != nil {
				duration := time.Duration(*req.Duration) * time.Second
				s.timers[swid] = time.AfterFunc(duration, func() {
					s.mutex.Lock()
					defer s.mutex.Unlock()
					delete(s.timers, swid)
					if err := sw.TurnOff(); err != nil {
						log.Printf("Failed to automatically turn off switch %s: %v", swid, err)
					}
					log.Printf("Automatically turned off switch %s after %s", swid, duration)
				})
			}
		case "off":
			if err := sw.TurnOff(); err != nil {
				s.sendJSONResponse(w, "error", "Failed to turn off switch", http.StatusInternalServerError)
				return
			}
		}
	}

	s.sendJSONResponse(w, "ok", "", http.StatusOK)
}

func (s *Server) switchStatusHandler(w http.ResponseWriter, r *http.Request) {
	switchIDStr := chi.URLParam(r, "id")

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if switchIDStr == "all" {
		// Get detailed state for all switches
		states, err := s.switches.GetDetailedState()
		if err != nil {
			log.Printf("Failed to get detailed switch states: %v", err)
			s.sendJSONResponse(w, "error", "Failed to get switch states", http.StatusInternalServerError)
			return
		}

		// Get summary state (true if all switches are on)
		summary, err := s.switches.GetState()
		if err != nil {
			log.Printf("Failed to get summary switch state: %v", err)
			s.sendJSONResponse(w, "error", "Failed to get switch state", http.StatusInternalServerError)
			return
		}

		response := switchStatusResponse{
			Status: "ok",
			Data: allSwitchesStatus{
				Count:   s.switches.CountSwitches(),
				States:  states,
				Summary: summary,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Handle single switch status - ID already validated by middleware
	id, _ := strconv.Atoi(switchIDStr)
	sw, _ := s.switches.GetSwitch(uint(id)) // Already validated by middleware

	state, err := sw.GetState()
	if err != nil {
		log.Printf("Failed to get state for switch %d: %v", id, err)
		s.sendJSONResponse(w, "error", "Failed to get switch state", http.StatusInternalServerError)
		return
	}

	response := switchStatusResponse{
		Status: "ok",
		Data: singleSwitchStatus{
			ID:    switchIDStr,
			State: state,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
