package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/larsks/airdancer/internal/piface"
)

var (
	// outputState stores the current state of the 8 relays.
	outputState uint8
	// mutex protects access to outputState.
	mutex sync.Mutex
	// pf is the PiFace interface.
	pf *piface.PiFace
)

type relayRequest struct {
	State string `json:"state"`
}

func relayHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is accepted", http.StatusMethodNotAllowed)
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 3 || parts[1] != "relay" {
		http.NotFound(w, r)
		return
	}

	id, err := strconv.Atoi(parts[2])
	if err != nil || id < 0 || id > 7 {
		http.Error(w, "Invalid relay ID", http.StatusBadRequest)
		return
	}

	var req relayRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Failed to decode request body", http.StatusBadRequest)
		return
	}

	mutex.Lock()
	defer mutex.Unlock()

	newState := outputState
	switch req.State {
	case "on":
		newState |= (1 << uint(id))
	case "off":
		newState &^= (1 << uint(id))
	default:
		http.Error(w, "Invalid state, must be 'on' or 'off'", http.StatusBadRequest)
		return
	}

	if err := pf.WriteOutputs(newState); err != nil {
		log.Printf("Failed to write outputs: %v", err)
		http.Error(w, "Failed to write to PiFace device", http.StatusInternalServerError)
		return
	}

	outputState = newState
	log.Printf("Set relay %d to %s, new state: 0b%08b", id, req.State, outputState)
	fmt.Fprintf(w, "OK")
}

func main() {
	var err error
	// Replace "/dev/spidev0.0" with your actual SPI port if different.
	pf, err = piface.NewPiFace("/dev/spidev0.0")
	if err != nil {
		log.Fatalf("Failed to initialize PiFace: %v", err)
	}
	defer pf.Close()

	// Initialize all outputs to off
	if err := pf.WriteOutputs(0); err != nil {
		log.Fatalf("Failed to initialize outputs: %v", err)
	}

	http.HandleFunc("/relay/", relayHandler)

	log.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
