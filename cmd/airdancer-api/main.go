package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/larsks/airdancer/internal/piface"
	"github.com/spf13/pflag"
)

var (
	// outputState stores the current state of the 8 relays.
	outputState uint8
	// mutex protects access to outputState.
	mutex sync.Mutex
	// pf is the PiFace interface.
	pf *piface.PiFace
	// timers stores active timers for relays.
	timers = make(map[int]*time.Timer)
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

func sendJSONResponse(w http.ResponseWriter, status string, message string, httpCode int, outputState uint8) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpCode)
	json.NewEncoder(w).Encode(jsonResponse{
		Status:      status,
		Message:     message,
		OutputState: outputState,
	})
}

func relayHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendJSONResponse(w, "error", "Only POST method is accepted", http.StatusMethodNotAllowed, outputState)
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 3 || parts[1] != "relay" {
		sendJSONResponse(w, "error", "Not found", http.StatusNotFound, outputState)
		return
	}

	var req relayRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSONResponse(w, "error", "Failed to decode request body", http.StatusBadRequest, outputState)
		return
	}

	mutex.Lock()
	defer mutex.Unlock()

	var relayIDs []int
	if parts[2] == "all" {
		for i := range 8 {
			relayIDs = append(relayIDs, i)
		}
	} else {
		id, err := strconv.Atoi(parts[2])
		if err != nil || id < 0 || id > 7 {
			sendJSONResponse(w, "error", "Invalid relay ID", http.StatusBadRequest, outputState)
			return
		}
		relayIDs = append(relayIDs, id)
	}

	newState := outputState
	for _, id := range relayIDs {
		// Cancel any existing timer for this relay
		if timer, ok := timers[id]; ok {
			timer.Stop()
			delete(timers, id)
		}

		switch req.State {
		case "on":
			newState |= (1 << uint(id))
			if req.Duration != nil {
				duration := time.Duration(*req.Duration) * time.Second
				timers[id] = time.AfterFunc(duration, func() {
					mutex.Lock()
					defer mutex.Unlock()
					delete(timers, id)
					turnOffState := outputState &^ (1 << uint(id))
					if err := pf.WriteOutputs(turnOffState); err != nil {
						log.Printf("Failed to automatically turn off relay %d: %v", id, err)
					} else {
						outputState = turnOffState
						log.Printf("Automatically turned off relay %d after %s", id, duration)
					}
				})
			}
		case "off":
			newState &^= (1 << uint(id))
		default:
			sendJSONResponse(w, "error", "Invalid state, must be 'on' or 'off'", http.StatusBadRequest, outputState)
			return
		}
	}

	if err := pf.WriteOutputs(newState); err != nil {
		log.Printf("Failed to write outputs: %v", err)
		sendJSONResponse(w, "error", "Failed to write to PiFace device", http.StatusInternalServerError, outputState)
		return
	}

	outputState = newState
	log.Printf("Set relays to %s, new state: 0b%08b", req.State, outputState)
	sendJSONResponse(w, "ok", "", http.StatusOK, outputState)
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	sendJSONResponse(w, "ok", "", http.StatusOK, outputState)
}

func getenvWithDefault(name string, defaultValue string) string {
	if val, ok := os.LookupEnv(name); ok {
		return val
	}

	return defaultValue
}

func main() {
	var err error
	var spidev string
	var listenAddress string
	var listenPortStr string

	pflag.StringVar(&spidev, "spidev", getenvWithDefault("AIRDANCER_SPIDEV", "/dev/spidev0.0"), "SPI device to use")
	pflag.StringVar(&listenAddress, "listen-address", getenvWithDefault("AIRDANCER_LISTEN_ADDRESS", ""), "Listen address for http server")
	pflag.StringVar(&listenPortStr, "listen-port", getenvWithDefault("AIRDANCER_LISTEN_PORT", "8080"), "Listen port for http server")
	pflag.Parse()

	listenPort, err := strconv.Atoi(listenPortStr)
	if err != nil {
		log.Fatalf("invalid listen port %q: %v", listenPortStr, err)
	}

	pf, err = piface.NewPiFace(spidev)
	if err != nil {
		log.Fatalf("Failed to initialize PiFace: %v", err)
	}
	defer pf.Close()

	// Initialize all outputs to off
	if err := pf.WriteOutputs(0); err != nil {
		log.Fatalf("Failed to initialize outputs: %v", err)
	}

	http.HandleFunc("/relay/", relayHandler)
	http.HandleFunc("/status", statusHandler)

	listenAddr := fmt.Sprintf("%s:%d", listenAddress, listenPort)
	log.Printf("Starting server on %s", listenAddr)
	if err := http.ListenAndServe(listenAddr, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
