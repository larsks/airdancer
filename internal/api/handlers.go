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
	Status      string `json:"status"`
	OutputState uint8  `json:"output_state"`
	Message     string `json:"message,omitempty"`
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
	var req switchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendJSONResponse(w, "error", "Failed to decode request body", http.StatusBadRequest)
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	switches, err := s.getSwitchesFromRequest(r)
	if err != nil {
		log.Printf("Failed to get list of switches: %v", err)
		s.sendJSONResponse(w, "error", "Failed to get list of switches", http.StatusInternalServerError)
	}

	for _, sw := range switches {
		swid := sw.String()

		if timer, ok := s.timers[swid]; ok {
			timer.Stop()
			delete(s.timers, swid)
		}

		switch req.State {
		case "on":
			if err := sw.TurnOn(); err != nil {
				s.sendJSONResponse(w, "error", "Failed to turn on switch", http.StatusBadRequest)
			}
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
				s.sendJSONResponse(w, "error", "Failed to turn off switch", http.StatusBadRequest)
			}
		default:
			s.sendJSONResponse(w, "error", "Invalid state, must be 'on' or 'off'", http.StatusBadRequest)
			return
		}
	}

	s.sendJSONResponse(w, "ok", "", http.StatusOK)
}

func (s *Server) switchStatusHandler(w http.ResponseWriter, r *http.Request) {
}
