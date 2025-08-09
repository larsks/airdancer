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
	switchStateDisabled switchState = "disabled"
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

func (s *Server) handleSwitchHelper(_ http.ResponseWriter, req *switchRequest, swid string, sw switchcollection.Switch) error {
	// Check if switch is disabled and reject operations other than status queries
	if sw.IsDisabled() {
		return fmt.Errorf("switch %s is disabled due to network connectivity issues", swid)
	}

	// Cancel any existing timer and task for this switch
	if err := s.cancelTasksAndTimers(swid); err != nil {
		return err
	}

	// Execute switch operation
	switch req.State {
	case switchStateOn:
		if err := sw.TurnOn(); err != nil {
			return fmt.Errorf("failed to turn on switch %s: %w", swid, err)
		}
		s.publishMQTTSwitchEvent(swid, "on")
	case switchStateOff:
		if err := sw.TurnOff(); err != nil {
			return fmt.Errorf("failed to turn off switch %s: %w", sw, err)
		}
		s.publishMQTTSwitchEvent(swid, "off")
	case switchStateToggle:
		var err error
		var state bool

		state, err = sw.GetState()
		if err != nil {
			return fmt.Errorf("failed to get switch state for switch %s: %w", sw, err)
		}

		if state {
			err = sw.TurnOff()
			if err == nil {
				s.publishSwitchStateChange(swid, false)
			}
		} else {
			err = sw.TurnOn()
			if err == nil {
				s.publishSwitchStateChange(swid, true)
			}
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

		task := NewBlinkTask(newBlinker)
		if err := s.taskManager.StartTask(swid, task); err != nil {
			return fmt.Errorf("failed to start blinker for %s: %w", swid, err)
		}
	case switchStateFlipflop:
		return fmt.Errorf("flipflop state is only supported for switch groups, not individual switches")
	}

	// Set up auto-off timer if duration specified
	if req.Duration != nil {
		duration := time.Duration(*req.Duration) * time.Second
		s.setupAutoOffTimer(swid, duration, sw)
	}

	return nil
}

func (s *Server) handleAllSwitches(w http.ResponseWriter, r *http.Request) {
	req, _ := r.Context().Value(switchRequestKey).(switchRequest)

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Cancel all existing timers and tasks
	if err := s.cancelAllTasksAndTimers(); err != nil {
		log.Printf("failed to cancel all tasks and timers: %v", err)
	}

	// Apply operation to all defined switches
	errorCollector := NewErrorCollector()
	for switchName, resolvedSwitch := range s.switches {
		// Skip disabled switches
		if resolvedSwitch.Switch.IsDisabled() {
			continue
		}
		if err := s.handleSwitchHelper(w, &req, switchName, resolvedSwitch.Switch); err != nil {
			errorCollector.Add(fmt.Sprintf("switch %s", switchName), err)
		}
	}

	if errorCollector.HasErrors() {
		s.sendError(w, errorCollector.Result("errors applying to all switches").Error(), http.StatusBadRequest)
		return
	}
	s.sendSuccess(w, req)
}

func (s *Server) handleSingleSwitch(w http.ResponseWriter, r *http.Request, switchName string) {
	// Get validated request directly from context
	req, _ := r.Context().Value(switchRequestKey).(switchRequest)

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// If there was an "all" timer running, turn off all switches when canceling it
	hadAllTimer := false
	if _, ok := s.timers["all"]; ok {
		hadAllTimer = true
	}

	// Cancel any "all switches" operations that might be running
	if err := s.cancelTasksAndTimers("all"); err != nil {
		s.sendError(w, fmt.Sprintf("Failed to cancel tasks and timers on all: %v", err), http.StatusInternalServerError)
		return
	}

	// Turn off all defined switches if we canceled an "all" timer
	if hadAllTimer {
		log.Printf("turning off all switches due to canceled 'all' timer")
		for _, resolvedSwitch := range s.switches {
			if err := resolvedSwitch.Switch.TurnOff(); err != nil {
				log.Printf("Failed to turn off switch %s during all timer cancellation: %v", resolvedSwitch.Name, err)
			}
		}
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
	defer s.mutex.Unlock()

	// Handle flipflop specially since it operates on the group as a whole
	if req.State == switchStateFlipflop {
		// Cancel any existing timer and task for this group
		if err := s.cancelTasksAndTimers(groupName); err != nil {
			s.sendError(w, fmt.Sprintf("failed to cancel task on group %s: %v", groupName, err), http.StatusInternalServerError)
			return
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

		task := NewFlipflopTask(newFlipflop)
		if err := s.taskManager.StartTask(groupName, task); err != nil {
			s.sendError(w, fmt.Sprintf("failed to start flipflop for group %s: %v", groupName, err), http.StatusBadRequest)
			return
		}

		// Set up auto-off timer if duration specified
		if req.Duration != nil {
			duration := time.Duration(*req.Duration) * time.Second
			// For flipflop, we just stop the task, don't turn switches off
			cleanup := func() {
				if err := s.taskManager.StopTask(groupName); err != nil {
					log.Printf("timer failed to stop task on group %s: %v", groupName, err)
				}
			}
			s.setupTimer(groupName, duration, cleanup)
		}

		s.sendSuccess(w, req)
		return
	}

	// Handle blink specially since it operates on the group as a whole
	if req.State == switchStateBlink {
		// Cancel any existing timer and task for this group
		if err := s.cancelTasksAndTimers(groupName); err != nil {
			s.sendError(w, fmt.Sprintf("failed to cancel task on group %s: %v", groupName, err), http.StatusInternalServerError)
			return
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

		task := NewBlinkTask(newBlinker)
		if err := s.taskManager.StartTask(groupName, task); err != nil {
			s.sendError(w, fmt.Sprintf("failed to start blinker for group %s: %v", groupName, err), http.StatusBadRequest)
			return
		}

		// Set up auto-off timer if duration specified
		if req.Duration != nil {
			duration := time.Duration(*req.Duration) * time.Second
			// For blink, we just stop the task, don't turn switches off
			cleanup := func() {
				if err := s.taskManager.StopTask(groupName); err != nil {
					log.Printf("timer failed to stop task on group %s: %v", groupName, err)
				}
			}
			s.setupTimer(groupName, duration, cleanup)
		}

		s.sendSuccess(w, req)
		return
	}

	// For all other states, first cancel any group-level activities
	if err := s.cancelTasksAndTimers(groupName); err != nil {
		s.sendError(w, fmt.Sprintf("failed to cancel task on group %s: %v", groupName, err), http.StatusInternalServerError)
		return
	}

	// Now apply operation to all switches in the group
	errorCollector := NewErrorCollector()
	for switchName, resolvedSwitch := range group.GetSwitches() {
		// Skip disabled switches
		if resolvedSwitch.Switch.IsDisabled() {
			continue
		}
		if err := s.handleSwitchHelper(w, &req, switchName, resolvedSwitch.Switch); err != nil {
			errorCollector.Add(fmt.Sprintf("switch %s", switchName), err)
		}
	}

	if errorCollector.HasErrors() {
		s.sendError(w, errorCollector.Result(fmt.Sprintf("errors applying to group %s", groupName)).Error(), http.StatusBadRequest)
		return
	}
	s.sendSuccess(w, req)
}

func (s *Server) getStatusForSwitch(switchName string, sw switchcollection.Switch) (*switchResponse, error) {
	swid := switchName

	// Check if switch is disabled first
	if sw.IsDisabled() {
		return &switchResponse{
			CurrentState: false,
			switchRequest: switchRequest{
				State: switchStateDisabled,
			},
		}, nil
	}

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

	if task, exists := s.taskManager.GetTask(swid); exists {
		if task.IsRunning() {
			if blinkTask, ok := task.(*BlinkTask); ok {
				period := blinkTask.GetPeriod()
				duty := blinkTask.GetDutyCycle()

				response.State = switchStateBlink
				response.Period = &period
				response.DutyCycle = &duty
			} else if task.Type() == TaskTypeFlipflop {
				response.State = switchStateFlipflop
			}
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
	defer s.mutex.Unlock()

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
		// Skip disabled switches when calculating group summary
		if resolvedSwitch.Switch.IsDisabled() {
			allOn = false
			break
		}
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
	if task, exists := s.taskManager.GetTask(groupName); exists {
		if task.IsRunning() {
			if task.Type() == TaskTypeBlink {
				response.State = switchStateBlink
			} else if task.Type() == TaskTypeFlipflop {
				response.State = switchStateFlipflop
			}
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

	if task, exists := s.taskManager.GetTask("all"); exists {
		if task.IsRunning() && task.Type() == TaskTypeBlink {
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

	if task, exists := s.taskManager.GetTask(groupName); exists {
		if task.IsRunning() {
			if task.Type() == TaskTypeBlink {
				response.State = switchStateBlink
			} else if task.Type() == TaskTypeFlipflop {
				response.State = switchStateFlipflop
			}
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
