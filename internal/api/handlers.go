package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

type relayRequest struct {
	State    string `json:"state"`
	Duration *int   `json:"duration,omitempty"`
}

type jsonResponse struct {
	Status      string `json:"status"`
	OutputState uint8  `json:"output_state"`
	Message     string `json:"message,omitempty"`
}

func (s *Server) sendJSONResponse(w http.ResponseWriter, status string, message string, httpCode int, outputState uint8) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpCode)
	json.NewEncoder(w).Encode(jsonResponse{
		Status:      status,
		Message:     message,
		OutputState: outputState,
	})
}

func (s *Server) relayHandler(w http.ResponseWriter, r *http.Request) {
	var req relayRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendJSONResponse(w, "error", "Failed to decode request body", http.StatusBadRequest, s.outputState)
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	var relayIDs []int
	relayIDStr := chi.URLParam(r, "id")
	if relayIDStr == "all" {
		for i := 0; i < 8; i++ {
			relayIDs = append(relayIDs, i)
		}
	} else {
		id, err := strconv.Atoi(relayIDStr)
		if err != nil || id < 0 || id > 7 {
			s.sendJSONResponse(w, "error", "Invalid relay ID", http.StatusBadRequest, s.outputState)
			return
		}
		relayIDs = append(relayIDs, id)
	}

	newState := s.outputState
	for _, id := range relayIDs {
		if timer, ok := s.timers[id]; ok {
			timer.Stop()
			delete(s.timers, id)
		}

		switch req.State {
		case "on":
			newState |= (1 << uint(id))
			if req.Duration != nil {
				duration := time.Duration(*req.Duration) * time.Second
				s.timers[id] = time.AfterFunc(duration, func() {
					s.mutex.Lock()
					defer s.mutex.Unlock()
					delete(s.timers, id)
					turnOffState := s.outputState &^ (1 << uint(id))
					if err := s.pf.WriteOutputs(turnOffState); err != nil {
						log.Printf("Failed to automatically turn off relay %d: %v", id, err)
					} else {
						s.outputState = turnOffState
						log.Printf("Automatically turned off relay %d after %s", id, duration)
					}
				})
			}
		case "off":
			newState &^= (1 << uint(id))
		default:
			s.sendJSONResponse(w, "error", "Invalid state, must be 'on' or 'off'", http.StatusBadRequest, s.outputState)
			return
		}
	}

	if err := s.pf.WriteOutputs(newState); err != nil {
		log.Printf("Failed to write outputs: %v", err)
		s.sendJSONResponse(w, "error", "Failed to write to PiFace device", http.StatusInternalServerError, s.outputState)
		return
	}

	s.outputState = newState
	log.Printf("Set relays to %s, new state: 0b%08b", req.State, s.outputState)
	s.sendJSONResponse(w, "ok", "", http.StatusOK, s.outputState)
}

func (s *Server) statusHandler(w http.ResponseWriter, r *http.Request) {
	s.sendJSONResponse(w, "ok", "", http.StatusOK, s.outputState)
}
