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
	"github.com/larsks/airdancer/internal/switchcollection"
)

type switchState string

const (
	switchStateOn    switchState = "on"
	switchStateOff               = "off"
	switchStateBlink             = "blink"
)

type (
	switchRequest struct {
		State     switchState `json:"state"`
		Duration  *int        `json:"duration,omitempty"`
		Period    *float64    `json:"period,omitempty"`
		DutyCycle *float64    `json:"dutyCycle,omitempty"`
	}

	switchResponse struct {
		switchRequest
		CurrentState switchState `json:"currentState"`
	}

	// Single response type that handles all cases
	APIResponse struct {
		Status  string `json:"status"`
		Message string `json:"message,omitempty"`
		Data    any    `json:"data,omitempty"`
	}

	multiSwitchResponse struct {
		Summary  bool `json:"summary"`
		Switches []switchResponse
	}
)

// Helper methods for responses
func (s *Server) sendSuccess(w http.ResponseWriter, data any) {
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

	if switchID == "all" {
		s.handleAllSwitches(w, r)
	} else {
		id, _ := strconv.Atoi(switchID)
		s.handleSingleSwitch(w, r, uint(id))
	}
}

func (s *Server) handleSwitchHelper(w http.ResponseWriter, req *switchRequest, swid string, sw switchcollection.Switch) error {
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
				return fmt.Errorf("failed to cancel blinker on %s: %w", swid, err)
			}
		}
		delete(s.blinkers, swid)
	}

	// Execute switch operation
	switch req.State {
	case switchStateOn:
		if err := sw.TurnOn(); err != nil {
			return fmt.Errorf("failed to turn on switch %s: %w", swid, err)
		}
	case switchStateOff:
		if err := sw.TurnOff(); err != nil {
			return fmt.Errorf("failed to turn off switch %s: %w", err)
		}
	case switchStateBlink:
		dutyCycle := 0.5
		if req.DutyCycle != nil {
			dutyCycle = *req.DutyCycle
		}

		newBlinker, err := blink.NewBlink(sw, *req.Period, dutyCycle)
		if err != nil {
			return fmt.Errorf("failed to create blinker for %s: %w", swid, err)
		}
		s.blinkers[swid] = newBlinker
		log.Printf("start blinker on %s", swid)
		if err := newBlinker.Start(); err != nil {
			return fmt.Errorf("failed to start blinker for %s: %w", swid, err)
		}
	}

	// Set up auto-off timer if duration specified
	if req.Duration != nil {
		duration := time.Duration(*req.Duration) * time.Second
		log.Printf("start timer on %s for %v", swid, duration)
		s.timers[swid] = &timerData{
			duration: duration,
			timer: time.AfterFunc(duration, func() {
				s.mutex.Lock()
				defer s.mutex.Unlock() //nolint:errcheck
				delete(s.timers, swid)

				if err := sw.TurnOff(); err != nil {
					log.Printf("timer failed to turn off switch %s: %v", swid, err)
				}
				log.Printf("timer expired for switch %s after %s", swid, duration)
			}),
		}
	}

	return nil
}

func (s *Server) handleAllSwitches(w http.ResponseWriter, r *http.Request) {
	req, _ := r.Context().Value(switchRequestKey).(switchRequest)

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

	if err := s.handleSwitchHelper(w, &req, "all", s.switches); err != nil {
		s.sendError(w, err.Error(), http.StatusBadRequest)
	}
	s.sendSuccess(w, req)
}

func (s *Server) handleSingleSwitch(w http.ResponseWriter, r *http.Request, id uint) {
	// Get validated request directly from context
	req, _ := r.Context().Value(switchRequestKey).(switchRequest)

	s.mutex.Lock()
	defer s.mutex.Unlock() //nolint:errcheck

	sw, _ := s.switches.GetSwitch(id)
	swid := sw.String()
	if err := s.handleSwitchHelper(w, &req, swid, sw); err != nil {
		s.sendError(w, err.Error(), http.StatusBadRequest)
	}
	s.sendSuccess(w, req)
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

	states := []map[string]any{}
	for i, bState := range boolStates {
		state := map[string]any{}
		if bState {
			state["state"] = switchStateOn
		} else {
			state["state"] = switchStateOff
		}

		swid := fmt.Sprintf("switch-%d", i)
		if blinker, ok := s.blinkers[swid]; ok && blinker.IsRunning() {
			state["state"] = switchStateBlink
			state["period"] = blinker.GetPeriod()
			state["dutyCycle"] = blinker.GetDutyCycle()
		}
		if timer, ok := s.timers[swid]; ok {
			state["duration"] = timer.duration
		}
		states = append(states, state)
	}

	data := map[string]any{
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

	data := map[string]any{
		"state": state,
	}

	if blinker, ok := s.blinkers[swid]; ok && blinker.IsRunning() {
		data["state"] = "blink"
		data["period"] = blinker.GetPeriod()
		data["dutyCycle"] = blinker.GetDutyCycle()
	}

	if timer, ok := s.timers[swid]; ok {
		data["duration"] = timer.duration
	}

	s.sendSuccess(w, data)
}

func (s *Server) listRoutesHandler(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{"routes": s.ListRoutes()}
	s.sendSuccess(w, data)
}
