package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/larsks/airdancer/internal/blink"
)

type switchRequest struct {
	State     string   `json:"state"`
	Duration  *int     `json:"duration,omitempty"`
	Period    *float64 `json:"period,omitempty"`
	DutyCycle *float64 `json:"dutyCycle,omitempty"`
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
		log.Printf("cancelling timer on %s", swid)
		timer.timer.Stop()
		delete(s.timers, swid)
	}

	// Cancel any existing blinker for all switches
	for swid, blinker := range s.blinkers {
		log.Printf("cancelling blinker on %s", swid)
		if err := blinker.Stop(); err != nil {
			log.Printf("failed to stop blinker on %s: %v", swid, err)
		}
		delete(s.blinkers, swid)
	}

	// Execute switch operation on all switches
	switch req.State {
	case "on":
		if err := s.switches.TurnOn(); err != nil {
			s.sendError(w, "Failed to turn on switches", http.StatusInternalServerError)
			return
		}
	case "off":
		if err := s.switches.TurnOff(); err != nil {
			s.sendError(w, "Failed to turn off switches", http.StatusInternalServerError)
			return
		}
	case "blink":
		s.sendError(w, "Blinking all switches is not supported", http.StatusBadRequest)
		return
	}

	// Set up auto-off timer if duration specified
	if req.Duration != nil {
		duration := time.Duration(*req.Duration) * time.Second
		swid := s.switches.String()
		log.Printf("start timer on all switches for %v", duration)
		s.timers[swid] = &timerData{
			expiresAt: time.Now().Add(duration),
			timer: time.AfterFunc(duration, func() {
				s.mutex.Lock()
				defer s.mutex.Unlock() //nolint:errcheck
				delete(s.timers, swid)
				if err := s.switches.TurnOff(); err != nil {
					log.Printf("failed to automatically turn off all switches: %v", err)
				}
				log.Printf("automatically turned off all switches after %s", duration)
			}),
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
		log.Printf("cancelling timer on %s", swid)
		timer.timer.Stop()
		delete(s.timers, swid)
	}

	// Stop any running blinker for this switch
	if blinker, ok := s.blinkers[swid]; ok {
		if blinker.IsRunning() {
			log.Printf("cancelling blinker on %s", swid)
			if err := blinker.Stop(); err != nil {
				s.sendError(w, fmt.Sprintf("Failed to stop blinker: %v", err), http.StatusInternalServerError)
				return
			}
		}
		delete(s.blinkers, swid)
	}

	// Execute switch operation
	switch req.State {
	case "on":
		if err := sw.TurnOn(); err != nil {
			s.sendError(w, "Failed to turn on switch", http.StatusInternalServerError)
			return
		}
	case "off":
		if err := sw.TurnOff(); err != nil {
			s.sendError(w, "Failed to turn off switch", http.StatusInternalServerError)
			return
		}
	case "blink":
		dutyCycle := 0.5
		if req.DutyCycle != nil {
			dutyCycle = *req.DutyCycle
		}

		newBlinker, err := blink.NewBlink(sw, *req.Period, dutyCycle)
		if err != nil {
			s.sendError(w, fmt.Sprintf("Failed to create blinker: %v", err), http.StatusInternalServerError)
			return
		}
		s.blinkers[swid] = newBlinker
		log.Printf("start blinker on %s", swid)
		if err := newBlinker.Start(); err != nil {
			s.sendError(w, fmt.Sprintf("Failed to start blinker: %v", err), http.StatusInternalServerError)
			return
		}
	}

	// Set up auto-off timer if duration specified
	if req.Duration != nil {
		duration := time.Duration(*req.Duration) * time.Second
		log.Printf("start timer on %s for %v", swid, duration)
		s.timers[swid] = &timerData{
			expiresAt: time.Now().Add(duration),
			timer: time.AfterFunc(duration, func() {
				s.mutex.Lock()
				defer s.mutex.Unlock() //nolint:errcheck
				delete(s.timers, swid)

				// Stop any running blinker for this switch
				if blinker, ok := s.blinkers[swid]; ok {
					if blinker.IsRunning() {
						log.Printf("cancelling blinker on %s", swid)
						if err := blinker.Stop(); err != nil {
							log.Printf("timer failed to stop blinker for switch %s: %v", swid, err)
						}
					}
					delete(s.blinkers, swid)
				}

				if err := sw.TurnOff(); err != nil {
					log.Printf("timer failed to turn off switch %s: %v", swid, err)
				}
				log.Printf("timer expired for switch %s after %s", swid, duration)
			}),
		}
	}

	s.sendSuccess(w, nil)
}

func (s *Server) switchStatusHandler(w http.ResponseWriter, r *http.Request) {
	switchIDStr := chi.URLParam(r, "id")

	s.mutex.Lock()
	defer s.mutex.Unlock() //nolint:errcheck

	if switchIDStr == "all" {
		s.handleAllSwitchesStatus(w)
	} else {
		id, _ := strconv.Atoi(switchIDStr) // No error check needed, already validated
		s.handleSingleSwitchStatus(w, uint(id), switchIDStr)
	}
}

func (s *Server) handleAllSwitchesStatus(w http.ResponseWriter) {
	// Get detailed state for all switches
	boolStates, err := s.switches.GetDetailedState()
	if err != nil {
		log.Printf("failed to get detailed switch states: %v", err)
		s.sendError(w, "Failed to get switch states", http.StatusInternalServerError)
		return
	}

	// Get summary state (true if all switches are on)
	summary, err := s.switches.GetState()
	if err != nil {
		log.Printf("failed to get summary switch state: %v", err)
		s.sendError(w, "Failed to get switch state", http.StatusInternalServerError)
		return
	}

	states := []map[string]interface{}{}
	for i, bState := range boolStates {
		state := map[string]interface{}{}
		if bState {
			state["state"] = "on"
		} else {
			state["state"] = "off"
		}

		swid := fmt.Sprintf("switch-%d", i)
		if blinker, ok := s.blinkers[swid]; ok && blinker.IsRunning() {
			state["state"] = "blink"
			state["period"] = blinker.GetPeriod()
			state["dutyCycle"] = blinker.GetDutyCycle()
		}
		if timer, ok := s.timers[swid]; ok {
			state["duration"] = int(time.Until(timer.expiresAt).Seconds())
		}
		states = append(states, state)
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

	swid := sw.String()

	state, err := sw.GetState()
	if err != nil {
		log.Printf("failed to get state for switch %d: %v", id, err)
		s.sendError(w, "Failed to get switch state", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"state": state,
	}

	if blinker, ok := s.blinkers[swid]; ok && blinker.IsRunning() {
		data["state"] = "blink"
		data["period"] = blinker.GetPeriod()
		data["dutyCycle"] = blinker.GetDutyCycle()
	}

	if timer, ok := s.timers[swid]; ok {
		data["duration"] = int(time.Until(timer.expiresAt).Seconds())
	}

	s.sendSuccess(w, data)
}

func (s *Server) listRoutesHandler(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{"routes": s.ListRoutes()}
	s.sendSuccess(w, data)
}

func (s *Server) blinkStatusHandler(w http.ResponseWriter, r *http.Request) {
	switchIDStr := chi.URLParam(r, "id")
	id, _ := strconv.Atoi(switchIDStr) // No error check needed, already validated

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Get switch - no error check needed, already validated by middleware
	sw, _ := s.switches.GetSwitch(uint(id))
	swid := sw.String()
	blinker, exists := s.blinkers[swid]

	data := map[string]any{
		"blinking": false,
	}

	if exists && blinker.IsRunning() {
		data["blinking"] = true
		data["period"] = blinker.GetPeriod()
		data["dutyCycle"] = blinker.GetDutyCycle()
	}

	s.sendSuccess(w, data)
}
