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

	// Cancel any existing timer for this relay
	if timer, ok := timers[id]; ok {
		timer.Stop()
		delete(timers, id)
	}

	newState := outputState
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

	listenAddr := fmt.Sprintf("%s:%d", listenAddress, listenPort)
	log.Printf("Starting server on %s", listenAddr)
	if err := http.ListenAndServe(listenAddr, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
