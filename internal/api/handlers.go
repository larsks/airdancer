package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/larsks/airdancer/internal/blink"
	"github.com/larsks/airdancer/internal/flipflop"
	"github.com/larsks/airdancer/internal/switchcollection"
)

type switchState string

const (
	switchStateOn       switchState = "on"
	switchStateOff      switchState = "off"
	switchStateBlink    switchState = "blink"
	switchStateToggle   switchState = "toggle"
	switchStateFlipflop switchState = "flipflop"
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
		CurrentState bool `json:"currentState"`
	}

	// Single response type that handles all cases
	APIResponse struct {
		Status  string `json:"status"`
		Message string `json:"message,omitempty"`
		Data    any    `json:"data,omitempty"`
	}

	multiSwitchResponse struct {
		Summary  bool                       `json:"summary"`
		State    switchState                `json:"state"`
		Count    uint                       `json:"count"`
		Switches map[string]*switchResponse `json:"switches"`
		Groups   map[string]*groupResponse  `json:"groups,omitempty"`
	}

	groupResponse struct {
		Switches []string    `json:"switches"`
		Summary  bool        `json:"summary"`
		State    switchState `json:"state"`
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
	switchName := chi.URLParam(r, "name")

	if switchName == "all" {
		s.handleAllSwitches(w, r)
	} else if group, exists := s.groups[switchName]; exists {
		s.handleGroupSwitch(w, r, switchName, group)
	} else {
		s.handleSingleSwitch(w, r, switchName)
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

	// Stop any running flipflop for this switch
	if flipflopInstance, ok := s.flipflops[swid]; ok {
		if flipflopInstance.IsRunning() {
			log.Printf("cancelling flipflop on %s", swid)
			if err := flipflopInstance.Stop(); err != nil {
				return fmt.Errorf("failed to cancel flipflop on %s: %w", swid, err)
			}
		}
		delete(s.flipflops, swid)
	}

	// Execute switch operation
	switch req.State {
	case switchStateOn:
		if err := sw.TurnOn(); err != nil {
			return fmt.Errorf("failed to turn on switch %s: %w", swid, err)
		}
	case switchStateOff:
		if err := sw.TurnOff(); err != nil {
			return fmt.Errorf("failed to turn off switch %s: %w", sw, err)
		}
	case switchStateToggle:
		var err error
		var state bool

		state, err = sw.GetState()
		if err != nil {
			return fmt.Errorf("failed to get switch state for switch %s: %w", sw, err)
		}

		if state {
			err = sw.TurnOff()
		} else {
			err = sw.TurnOn()
		}

		if err != nil {
			return fmt.Errorf("failed to toggle switch %s: %w", sw, err)
		}

		// no duration when using "toggle"
		return nil
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
	case switchStateFlipflop:
		return fmt.Errorf("flipflop state is only supported for switch groups, not individual switches")
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

				// Stop any running blinker for this switch
				if blinker, ok := s.blinkers[swid]; ok {
					if err := blinker.Stop(); err != nil {
						log.Printf("timer failed to stop blinker on switch %s: %v", swid, err)
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

	// Cancel any existing flipflop for all switches
	for swid, flipflopInstance := range s.flipflops {
		log.Printf("cancelling flipflop on %s", swid)
		if err := flipflopInstance.Stop(); err != nil {
			log.Printf("failed to stop flipflop on %s: %v", swid, err)
		}
		delete(s.flipflops, swid)
	}

	// Apply operation to all defined switches
	var errors []error
	for switchName, resolvedSwitch := range s.switches {
		if err := s.handleSwitchHelper(w, &req, switchName, resolvedSwitch.Switch); err != nil {
			errors = append(errors, fmt.Errorf("switch %s: %w", switchName, err))
		}
	}

	if len(errors) > 0 {
		s.sendError(w, fmt.Sprintf("errors applying to all switches: %v", errors), http.StatusBadRequest)
		return
	}
	s.sendSuccess(w, req)
}

func (s *Server) handleSingleSwitch(w http.ResponseWriter, r *http.Request, switchName string) {
	// Get validated request directly from context
	req, _ := r.Context().Value(switchRequestKey).(switchRequest)

	s.mutex.Lock()
	defer s.mutex.Unlock() //nolint:errcheck

	// Cancel any "all switches" operations that might be running
	if blinker, ok := s.blinkers["all"]; ok {
		if blinker.IsRunning() {
			log.Printf("cancelling blinker on all switches")
			if err := blinker.Stop(); err != nil {
				s.sendError(w, fmt.Sprintf("Failed to cancel blinker on all: %v", err), http.StatusInternalServerError)
				return
			}
			delete(s.blinkers, "all")
		}
	}

	if timer, ok := s.timers["all"]; ok {
		log.Printf("cancelling timer on all switches")
		timer.timer.Stop()
		// Turn off all defined switches when cancelling "all" timer
		for _, resolvedSwitch := range s.switches {
			if err := resolvedSwitch.Switch.TurnOff(); err != nil {
				log.Printf("Failed to turn off switch %s during all timer cancellation: %v", resolvedSwitch.Name, err)
			}
		}
		delete(s.timers, "all")
	}

	resolvedSwitch, exists := s.switches[switchName]
	if !exists {
		s.sendError(w, fmt.Sprintf("Switch %s not found", switchName), http.StatusNotFound)
		return
	}

	if err := s.handleSwitchHelper(w, &req, switchName, resolvedSwitch.Switch); err != nil {
		s.sendError(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.sendSuccess(w, req)
}

func (s *Server) handleGroupSwitch(w http.ResponseWriter, r *http.Request, groupName string, group *SwitchGroup) {
	req, _ := r.Context().Value(switchRequestKey).(switchRequest)

	s.mutex.Lock()
	defer s.mutex.Unlock() //nolint:errcheck

	// Handle flipflop specially since it operates on the group as a whole
	if req.State == switchStateFlipflop {
		// Cancel any existing timer for this group
		if timer, ok := s.timers[groupName]; ok {
			log.Printf("cancelling timer on group %s", groupName)
			timer.timer.Stop()
			delete(s.timers, groupName)
		}

		// Stop any running blinker for this group
		if blinker, ok := s.blinkers[groupName]; ok {
			if blinker.IsRunning() {
				log.Printf("cancelling blinker on group %s", groupName)
				if err := blinker.Stop(); err != nil {
					s.sendError(w, fmt.Sprintf("failed to cancel blinker on group %s: %v", groupName, err), http.StatusInternalServerError)
					return
				}
			}
			delete(s.blinkers, groupName)
		}

		// Stop any running flipflop for this group
		if flipflopInstance, ok := s.flipflops[groupName]; ok {
			if flipflopInstance.IsRunning() {
				log.Printf("cancelling flipflop on group %s", groupName)
				if err := flipflopInstance.Stop(); err != nil {
					s.sendError(w, fmt.Sprintf("failed to cancel flipflop on group %s: %v", groupName, err), http.StatusInternalServerError)
					return
				}
			}
			delete(s.flipflops, groupName)
		}

		// Create list of switches for the flipflop
		var switches []switchcollection.Switch
		for _, resolvedSwitch := range group.GetSwitches() {
			switches = append(switches, resolvedSwitch.Switch)
		}

		dutyCycle := 0.5
		if req.DutyCycle != nil {
			dutyCycle = *req.DutyCycle
		}

		newFlipflop, err := flipflop.NewFlipflop(switches, *req.Period, dutyCycle)
		if err != nil {
			s.sendError(w, fmt.Sprintf("failed to create flipflop for group %s: %v", groupName, err), http.StatusBadRequest)
			return
		}

		s.flipflops[groupName] = newFlipflop
		log.Printf("start flipflop on group %s", groupName)
		if err := newFlipflop.Start(); err != nil {
			s.sendError(w, fmt.Sprintf("failed to start flipflop for group %s: %v", groupName, err), http.StatusBadRequest)
			return
		}

		// Set up auto-off timer if duration specified
		if req.Duration != nil {
			duration := time.Duration(*req.Duration) * time.Second
			log.Printf("start timer on group %s for %v", groupName, duration)
			s.timers[groupName] = &timerData{
				duration: duration,
				timer: time.AfterFunc(duration, func() {
					s.mutex.Lock()
					defer s.mutex.Unlock() //nolint:errcheck
					delete(s.timers, groupName)

					if flipflopInstance, ok := s.flipflops[groupName]; ok {
						if err := flipflopInstance.Stop(); err != nil {
							log.Printf("timer failed to stop flipflop on group %s: %v", groupName, err)
						}
						delete(s.flipflops, groupName)
					}
					log.Printf("timer expired for group %s after %s", groupName, duration)
				}),
			}
		}

		s.sendSuccess(w, req)
		return
	}

	// Handle blink specially since it operates on the group as a whole
	if req.State == switchStateBlink {
		// Cancel any existing timer for this group
		if timer, ok := s.timers[groupName]; ok {
			log.Printf("cancelling timer on group %s", groupName)
			timer.timer.Stop()
			delete(s.timers, groupName)
		}

		// Stop any running blinker for this group
		if blinker, ok := s.blinkers[groupName]; ok {
			if blinker.IsRunning() {
				log.Printf("cancelling blinker on group %s", groupName)
				if err := blinker.Stop(); err != nil {
					s.sendError(w, fmt.Sprintf("failed to cancel blinker on group %s: %v", groupName, err), http.StatusInternalServerError)
					return
				}
			}
			delete(s.blinkers, groupName)
		}

		// Stop any running flipflop for this group
		if flipflopInstance, ok := s.flipflops[groupName]; ok {
			if flipflopInstance.IsRunning() {
				log.Printf("cancelling flipflop on group %s", groupName)
				if err := flipflopInstance.Stop(); err != nil {
					s.sendError(w, fmt.Sprintf("failed to cancel flipflop on group %s: %v", groupName, err), http.StatusInternalServerError)
					return
				}
			}
			delete(s.flipflops, groupName)
		}

		dutyCycle := 0.5
		if req.DutyCycle != nil {
			dutyCycle = *req.DutyCycle
		}

		newBlinker, err := blink.NewBlink(group, *req.Period, dutyCycle)
		if err != nil {
			s.sendError(w, fmt.Sprintf("failed to create blinker for group %s: %v", groupName, err), http.StatusBadRequest)
			return
		}

		s.blinkers[groupName] = newBlinker
		log.Printf("start blinker on group %s", groupName)
		if err := newBlinker.Start(); err != nil {
			s.sendError(w, fmt.Sprintf("failed to start blinker for group %s: %v", groupName, err), http.StatusBadRequest)
			return
		}

		// Set up auto-off timer if duration specified
		if req.Duration != nil {
			duration := time.Duration(*req.Duration) * time.Second
			log.Printf("start timer on group %s for %v", groupName, duration)
			s.timers[groupName] = &timerData{
				duration: duration,
				timer: time.AfterFunc(duration, func() {
					s.mutex.Lock()
					defer s.mutex.Unlock() //nolint:errcheck
					delete(s.timers, groupName)

					if blinker, ok := s.blinkers[groupName]; ok {
						if err := blinker.Stop(); err != nil {
							log.Printf("timer failed to stop blinker on group %s: %v", groupName, err)
						}
						delete(s.blinkers, groupName)
					}
					log.Printf("timer expired for group %s after %s", groupName, duration)
				}),
			}
		}

		s.sendSuccess(w, req)
		return
	}

	// For all other states, first cancel any group-level activities
	// Cancel any existing timer for this group
	if timer, ok := s.timers[groupName]; ok {
		log.Printf("cancelling timer on group %s", groupName)
		timer.timer.Stop()
		delete(s.timers, groupName)
	}

	// Stop any running blinker for this group
	if blinker, ok := s.blinkers[groupName]; ok {
		if blinker.IsRunning() {
			log.Printf("cancelling blinker on group %s", groupName)
			if err := blinker.Stop(); err != nil {
				s.sendError(w, fmt.Sprintf("failed to cancel blinker on group %s: %v", groupName, err), http.StatusInternalServerError)
				return
			}
		}
		delete(s.blinkers, groupName)
	}

	// Stop any running flipflop for this group
	if flipflopInstance, ok := s.flipflops[groupName]; ok {
		if flipflopInstance.IsRunning() {
			log.Printf("cancelling flipflop on group %s", groupName)
			if err := flipflopInstance.Stop(); err != nil {
				s.sendError(w, fmt.Sprintf("failed to cancel flipflop on group %s: %v", groupName, err), http.StatusInternalServerError)
				return
			}
		}
		delete(s.flipflops, groupName)
	}

	// Now apply operation to all switches in the group
	var errors []error
	for switchName, resolvedSwitch := range group.GetSwitches() {
		if err := s.handleSwitchHelper(w, &req, switchName, resolvedSwitch.Switch); err != nil {
			errors = append(errors, fmt.Errorf("switch %s: %w", switchName, err))
		}
	}

	if len(errors) > 0 {
		s.sendError(w, fmt.Sprintf("errors applying to group %s: %v", groupName, errors), http.StatusBadRequest)
		return
	}
	s.sendSuccess(w, req)
}

func (s *Server) getStatusForSwitch(switchName string, sw switchcollection.Switch) (*switchResponse, error) {
	swid := switchName
	currentState, err := sw.GetState()
	if err != nil {
		return nil, fmt.Errorf("failed to get state for switch %s: %w", sw, err)
	}

	response := switchResponse{
		CurrentState: currentState,
	}

	response.State = switchStateOn
	if !currentState {
		response.State = switchStateOff
	}

	if blinker, ok := s.blinkers[swid]; ok {
		if blinker.IsRunning() {
			period := blinker.GetPeriod()
			duty := blinker.GetDutyCycle()

			response.State = switchStateBlink
			response.Period = &period
			response.DutyCycle = &duty
		}
	}

	if timer, ok := s.timers[swid]; ok {
		duration := int(timer.duration / time.Second)
		response.Duration = &duration
	}

	return &response, nil
}

func (s *Server) switchStatusHandler(w http.ResponseWriter, r *http.Request) {
	switchName := chi.URLParam(r, "name")

	s.mutex.Lock()
	defer s.mutex.Unlock() //nolint:errcheck

	if switchName == "all" {
		s.handleAllSwitchesStatus(w)
	} else if group, exists := s.groups[switchName]; exists {
		s.handleGroupSwitchStatus(w, switchName, group)
	} else {
		s.handleSingleSwitchStatus(w, switchName)
	}
}

func (s *Server) getStatusForGroup(groupName string, group *SwitchGroup) (*groupResponse, error) {
	// Get list of switch names in the group
	switchNames := make([]string, 0, len(group.GetSwitches()))
	for switchName := range group.GetSwitches() {
		switchNames = append(switchNames, switchName)
	}

	// Calculate summary state (true if all switches in group are on)
	allOn := true
	for _, resolvedSwitch := range group.GetSwitches() {
		currentState, err := resolvedSwitch.Switch.GetState()
		if err != nil {
			return nil, fmt.Errorf("failed to get state for switch %s in group %s: %w", resolvedSwitch.Name, groupName, err)
		}
		if !currentState {
			allOn = false
			break
		}
	}

	response := &groupResponse{
		Switches: switchNames,
		Summary:  allOn,
		State:    switchStateOff,
	}

	if allOn {
		response.State = switchStateOn
	}

	// Check for group-level activities
	if blinker, ok := s.blinkers[groupName]; ok {
		if blinker.IsRunning() {
			response.State = switchStateBlink
		}
	}

	if flipflopInstance, ok := s.flipflops[groupName]; ok {
		if flipflopInstance.IsRunning() {
			response.State = switchStateFlipflop
		}
	}

	return response, nil
}

func (s *Server) handleAllSwitchesStatus(w http.ResponseWriter) {
	switchCount := uint(len(s.switches))
	response := multiSwitchResponse{
		Count:    switchCount,
		Switches: make(map[string]*switchResponse),
		Groups:   make(map[string]*groupResponse),
	}

	// Calculate summary state (true if all defined switches are on)
	allOn := true

	for switchName, resolvedSwitch := range s.switches {
		switchStatus, err := s.getStatusForSwitch(switchName, resolvedSwitch.Switch)
		if err != nil {
			s.sendError(w, fmt.Sprintf("Failed to get status for switch %s: %v", switchName, err), http.StatusBadRequest)
			return
		}

		// Store switch status with its name as the key
		response.Switches[switchName] = switchStatus

		if !switchStatus.CurrentState {
			allOn = false
		}
	}

	response.Summary = allOn
	response.State = switchStateOff
	if allOn {
		response.State = switchStateOn
	}

	if blinker, ok := s.blinkers["all"]; ok {
		if blinker.IsRunning() {
			response.State = switchStateBlink
		}
	}

	// Populate group information
	for groupName, group := range s.groups {
		groupStatus, err := s.getStatusForGroup(groupName, group)
		if err != nil {
			s.sendError(w, fmt.Sprintf("Failed to get status for group %s: %v", groupName, err), http.StatusBadRequest)
			return
		}
		response.Groups[groupName] = groupStatus
	}

	s.sendSuccess(w, response)
}

func (s *Server) handleGroupSwitchStatus(w http.ResponseWriter, groupName string, group *SwitchGroup) {
	switchCount := group.CountSwitches()
	response := multiSwitchResponse{
		Count:    switchCount,
		Switches: make(map[string]*switchResponse),
	}

	// Calculate summary state (true if all switches in group are on)
	allOn := true

	for switchName, resolvedSwitch := range group.GetSwitches() {
		switchStatus, err := s.getStatusForSwitch(switchName, resolvedSwitch.Switch)
		if err != nil {
			s.sendError(w, fmt.Sprintf("Failed to get status for switch %s in group %s: %v", switchName, groupName, err), http.StatusBadRequest)
			return
		}

		// Store switch status with its name as the key
		response.Switches[switchName] = switchStatus

		if !switchStatus.CurrentState {
			allOn = false
		}
	}

	response.Summary = allOn
	response.State = switchStateOff
	if allOn {
		response.State = switchStateOn
	}

	if blinker, ok := s.blinkers[groupName]; ok {
		if blinker.IsRunning() {
			response.State = switchStateBlink
		}
	}

	if flipflopInstance, ok := s.flipflops[groupName]; ok {
		if flipflopInstance.IsRunning() {
			response.State = switchStateFlipflop
		}
	}

	s.sendSuccess(w, response)
}

func (s *Server) handleSingleSwitchStatus(w http.ResponseWriter, switchName string) {
	// Check if switch exists
	resolvedSwitch, exists := s.switches[switchName]
	if !exists {
		s.sendError(w, fmt.Sprintf("Switch %s not found", switchName), http.StatusNotFound)
		return
	}

	response, err := s.getStatusForSwitch(switchName, resolvedSwitch.Switch)
	if err != nil {
		s.sendError(w, fmt.Sprintf("Failed to get status for switch %s: %v", switchName, err), http.StatusBadRequest)
		return
	}

	s.sendSuccess(w, response)
}

func (s *Server) listRoutesHandler(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{"routes": s.ListRoutes()}
	s.sendSuccess(w, data)
}
