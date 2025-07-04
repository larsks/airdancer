package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/larsks/airdancer/internal/blink"
)

type blinkRequest struct {
	State  string   `json:"state"`
	Period *float64 `json:"period,omitempty"`
}

type blinkRequestKeyType int

const blinkRequestKey blinkRequestKeyType = iota

func (s *Server) validateBlinkRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req blinkRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.sendError(w, "Invalid JSON request", http.StatusBadRequest)
			return
		}

		if req.State != "on" && req.State != "off" {
			s.sendError(w, "Invalid state; must be 'on' or 'off'", http.StatusBadRequest)
			return
		}

		if req.State == "on" && req.Period == nil {
			s.sendError(w, "Period is required when turning blink on", http.StatusBadRequest)
			return
		}

		if req.Period != nil && *req.Period <= 0 {
			s.sendError(w, "Period must be a positive value", http.StatusBadRequest)
			return
		}

		ctx := context.WithValue(r.Context(), blinkRequestKey, req)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) blinkHandler(w http.ResponseWriter, r *http.Request) {
	switchID := chi.URLParam(r, "id")
	id, err := strconv.Atoi(switchID)
	if err != nil {
		s.sendError(w, "Invalid switch ID", http.StatusBadRequest)
		return
	}

	req, ok := r.Context().Value(blinkRequestKey).(blinkRequest)
	if !ok {
		s.sendError(w, "Internal error: missing request data", http.StatusInternalServerError)
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	sw, err := s.switches.GetSwitch(uint(id))
	if err != nil {
		s.sendError(w, "Switch not found", http.StatusNotFound)
		return
	}

	swid := sw.String()

	// Stop any running timer for this switch
	if timer, ok := s.timers[swid]; ok {
		log.Printf("Cancelling timer on %s", swid)
		timer.Stop()
		delete(s.timers, swid)
	}

	blinker, blinkerExists := s.blinkers[swid]

	switch req.State {
	case "on":
		if blinkerExists && blinker.IsRunning() {
			if req.Period != nil && blinker.GetPeriod() != *req.Period {
				if err := blinker.Stop(); err != nil {
					s.sendError(w, fmt.Sprintf("Failed to stop existing blinker: %v", err), http.StatusInternalServerError)
					return
				}
				delete(s.blinkers, swid)
			}
		}

		newBlinker, err := blink.NewBlink(sw, *req.Period)
		if err != nil {
			s.sendError(w, fmt.Sprintf("Failed to create blinker: %v", err), http.StatusInternalServerError)
			return
		}
		s.blinkers[swid] = newBlinker
		if err := newBlinker.Start(); err != nil {
			s.sendError(w, fmt.Sprintf("Failed to start blinker: %v", err), http.StatusInternalServerError)
			return
		}

	case "off":
		if blinkerExists && blinker.IsRunning() {
			if err := blinker.Stop(); err != nil {
				s.sendError(w, fmt.Sprintf("Failed to stop blinker: %v", err), http.StatusInternalServerError)
				return
			}
			delete(s.blinkers, swid)
		}
	}

	s.sendSuccess(w, nil)
}
