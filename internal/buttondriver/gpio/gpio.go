package gpio

import (
	"fmt"
	"log"
	"sync"
	"time"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/host/v3"
)

// ButtonEvent represents a button press or release event
type ButtonEvent struct {
	Pin       string
	Pressed   bool
	Timestamp time.Time
}

// PullMode represents the pull resistor configuration
type PullMode int

const (
	PullNone PullMode = iota
	PullUp
	PullDown
	PullAuto // Automatically choose based on polarity
)

// ButtonDriver manages GPIO button monitoring with debouncing
type ButtonDriver struct {
	pins          map[string]*ButtonPin
	eventChannel  chan ButtonEvent
	stopChannel   chan struct{}
	wg            sync.WaitGroup
	debounceDelay time.Duration
	polarity      gpio.Level // gpio.High for active-high, gpio.Low for active-low
	pullMode      PullMode
}

// ButtonPin represents a single GPIO pin configured as a button
type ButtonPin struct {
	pin           gpio.PinIO
	name          string
	lastState     bool
	currentState  bool
	lastDebounce  time.Time
	stateReported bool
	driver        *ButtonDriver
	mutex         sync.Mutex
}

// NewButtonDriver creates a new GPIO button driver
func NewButtonDriver(debounceDelay time.Duration, activeHigh bool, pullMode PullMode) (*ButtonDriver, error) {
	if _, err := host.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize periph.io: %w", err)
	}

	polarity := gpio.High
	if !activeHigh {
		polarity = gpio.Low
	}

	return &ButtonDriver{
		pins:          make(map[string]*ButtonPin),
		eventChannel:  make(chan ButtonEvent, 100),
		stopChannel:   make(chan struct{}),
		debounceDelay: debounceDelay,
		polarity:      polarity,
		pullMode:      pullMode,
	}, nil
}

// AddPin adds a GPIO pin to be monitored as a button
func (bd *ButtonDriver) AddPin(pinName string) error {
	pin := gpioreg.ByName(pinName)
	if pin == nil {
		return fmt.Errorf("pin %s not found", pinName)
	}

	// Configure pin as input with pull resistor
	pull := bd.getPullResistor()
	if err := pin.In(pull, gpio.BothEdges); err != nil {
		return fmt.Errorf("failed to configure pin %s as input: %w", pinName, err)
	}

	initialState := bd.readButtonState(pin)
	buttonPin := &ButtonPin{
		pin:           pin,
		name:          pinName,
		driver:        bd,
		lastState:     initialState,
		currentState:  initialState,
		stateReported: true, // Initial state is considered "reported"
	}

	bd.pins[pinName] = buttonPin
	log.Printf("Added GPIO button pin: %s (pull: %s)", pinName, bd.getPullString())
	return nil
}

// Start begins monitoring all configured pins
func (bd *ButtonDriver) Start() error {
	if len(bd.pins) == 0 {
		return fmt.Errorf("no pins configured")
	}

	for _, buttonPin := range bd.pins {
		bd.wg.Add(1)
		go bd.monitorPin(buttonPin)
	}

	log.Printf("Started monitoring %d GPIO button pins", len(bd.pins))
	return nil
}

// Stop stops monitoring all pins
func (bd *ButtonDriver) Stop() {
	close(bd.stopChannel)
	bd.wg.Wait()
	close(bd.eventChannel)
	log.Printf("Stopped GPIO button monitoring")
}

// Events returns the channel for receiving button events
func (bd *ButtonDriver) Events() <-chan ButtonEvent {
	return bd.eventChannel
}

// monitorPin monitors a single GPIO pin for button events
func (bd *ButtonDriver) monitorPin(buttonPin *ButtonPin) {
	defer bd.wg.Done()

	// Initial state is already set in AddPin

	ticker := time.NewTicker(1 * time.Millisecond) // Check every 1ms
	defer ticker.Stop()

	for {
		select {
		case <-bd.stopChannel:
			return
		case <-ticker.C:
			bd.checkPinState(buttonPin)
		}
	}
}

// checkPinState checks the current state of a pin and handles debouncing
func (bd *ButtonDriver) checkPinState(buttonPin *ButtonPin) {
	currentState := bd.readButtonState(buttonPin.pin)
	now := time.Now()

	buttonPin.mutex.Lock()
	defer buttonPin.mutex.Unlock()

	// Check if state has changed from what we're currently tracking
	if currentState != buttonPin.currentState {
		// State changed, start debounce timer
		buttonPin.currentState = currentState
		buttonPin.lastDebounce = now
		buttonPin.stateReported = false
		return
	}

	// State is stable, check if debounce period has elapsed and we haven't reported this state yet
	if !buttonPin.stateReported && now.Sub(buttonPin.lastDebounce) >= bd.debounceDelay {
		// Debounce period has elapsed and this is a new stable state
		// Check if this is actually a change from the last reported state
		if currentState != buttonPin.lastState {
			buttonPin.lastState = currentState
			buttonPin.stateReported = true

			// Send event
			event := ButtonEvent{
				Pin:       buttonPin.name,
				Pressed:   currentState,
				Timestamp: now,
			}

			select {
			case bd.eventChannel <- event:
			default:
				log.Printf("Warning: event channel full, dropping event for pin %s", buttonPin.name)
			}
		} else {
			// State is the same as last reported, just mark as reported
			buttonPin.stateReported = true
		}
	}
}

// readButtonState reads the current logical state of a button pin
func (bd *ButtonDriver) readButtonState(pin gpio.PinIO) bool {
	level := pin.Read()
	return level == bd.polarity
}

// GetPins returns a list of configured pin names
func (bd *ButtonDriver) GetPins() []string {
	pins := make([]string, 0, len(bd.pins))
	for name := range bd.pins {
		pins = append(pins, name)
	}
	return pins
}

// GetDebounceDelay returns the current debounce delay
func (bd *ButtonDriver) GetDebounceDelay() time.Duration {
	return bd.debounceDelay
}

// SetDebounceDelay sets the debounce delay
func (bd *ButtonDriver) SetDebounceDelay(delay time.Duration) {
	bd.debounceDelay = delay
}

// getPullResistor returns the appropriate pull resistor configuration
func (bd *ButtonDriver) getPullResistor() gpio.Pull {
	switch bd.pullMode {
	case PullUp:
		return gpio.PullUp
	case PullDown:
		return gpio.PullDown
	case PullAuto:
		// Auto mode: choose pull resistor based on polarity
		if bd.polarity == gpio.High {
			return gpio.PullDown // Active-high buttons need pull-down
		}
		return gpio.PullUp // Active-low buttons need pull-up
	case PullNone:
		fallthrough
	default:
		return gpio.PullNoChange
	}
}

// getPullString returns a human-readable string for the pull resistor configuration
func (bd *ButtonDriver) getPullString() string {
	switch bd.pullMode {
	case PullUp:
		return "up"
	case PullDown:
		return "down"
	case PullAuto:
		if bd.polarity == gpio.High {
			return "down (auto)"
		}
		return "up (auto)"
	case PullNone:
		fallthrough
	default:
		return "none"
	}
}

// GetPullMode returns the current pull resistor mode
func (bd *ButtonDriver) GetPullMode() PullMode {
	return bd.pullMode
}