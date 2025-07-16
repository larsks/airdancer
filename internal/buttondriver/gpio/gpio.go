package gpio

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/larsks/airdancer/internal/buttondriver/common"
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/host/v3"
)

// Legacy ButtonEvent type for backward compatibility
// Deprecated: Use common.ButtonEvent instead
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
	pins            map[string]*ButtonPin
	eventChannel    chan common.ButtonEvent
	stopChannel     chan struct{}
	wg              sync.WaitGroup
	debounceDelay   time.Duration
	defaultPullMode PullMode
	started         bool
	mutex           sync.RWMutex
}

// ButtonPin represents a single GPIO pin configured as a button
type ButtonPin struct {
	pin           gpio.PinIO
	name          string
	pinName       string
	lastState     bool
	currentState  bool
	lastDebounce  time.Time
	stateReported bool
	polarity      gpio.Level
	driver        *ButtonDriver
	mutex         sync.Mutex
}

// NewButtonDriver creates a new GPIO button driver
func NewButtonDriver(debounceDelay time.Duration, defaultPullMode PullMode) (*ButtonDriver, error) {
	if _, err := host.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize periph.io: %w", err)
	}

	return &ButtonDriver{
		pins:            make(map[string]*ButtonPin),
		eventChannel:    make(chan common.ButtonEvent, 100),
		stopChannel:     make(chan struct{}),
		debounceDelay:   debounceDelay,
		defaultPullMode: defaultPullMode,
	}, nil
}

// NewButtonDriverWithDefaults creates a new GPIO button driver with default settings
func NewButtonDriverWithDefaults() (*ButtonDriver, error) {
	return NewButtonDriver(50*time.Millisecond, PullAuto)
}

// AddButton adds a button to be monitored (implements common.ButtonDriver interface)
func (bd *ButtonDriver) AddButton(buttonSpec interface{}) error {
	bd.mutex.Lock()
	defer bd.mutex.Unlock()

	if bd.started {
		return fmt.Errorf("cannot add buttons after driver has started")
	}

	spec, ok := buttonSpec.(*GPIOButtonSpec)
	if !ok {
		return fmt.Errorf("invalid button spec type, expected *GPIOButtonSpec")
	}

	if err := spec.Validate(); err != nil {
		return fmt.Errorf("invalid button spec: %w", err)
	}

	// Check if button already exists
	if _, exists := bd.pins[spec.Name]; exists {
		return fmt.Errorf("button %s already exists", spec.Name)
	}

	pin := gpioreg.ByName(spec.Pin)
	if pin == nil {
		return fmt.Errorf("pin %s not found", spec.Pin)
	}

	// Determine pull resistor configuration
	pullMode := spec.PullMode
	if pullMode == PullAuto {
		pullMode = bd.defaultPullMode
	}

	// Configure pin as input with pull resistor
	pull := bd.getPullResistor(pullMode, spec.ActiveHigh)
	if err := pin.In(pull, gpio.BothEdges); err != nil {
		return fmt.Errorf("failed to configure pin %s as input: %w", spec.Pin, err)
	}

	// Determine polarity
	polarity := gpio.High
	if !spec.ActiveHigh {
		polarity = gpio.Low
	}

	initialState := bd.readButtonState(pin, polarity)
	buttonPin := &ButtonPin{
		pin:           pin,
		name:          spec.Name,
		pinName:       spec.Pin,
		driver:        bd,
		lastState:     initialState,
		currentState:  initialState,
		stateReported: true, // Initial state is considered "reported"
		polarity:      polarity,
	}

	bd.pins[spec.Name] = buttonPin
	log.Printf("Added GPIO button: %s on pin %s (pull: %s)", spec.Name, spec.Pin, bd.getPullString(pullMode, spec.ActiveHigh))
	return nil
}

// AddPin adds a GPIO pin to be monitored as a button (legacy method)
// Deprecated: Use AddButton with GPIOButtonSpec instead
func (bd *ButtonDriver) AddPin(pinName string) error {
	spec := &GPIOButtonSpec{
		Name:       pinName,
		Pin:        pinName,
		ActiveHigh: true,
		PullMode:   bd.defaultPullMode,
	}
	return bd.AddButton(spec)
}

// Start begins monitoring all configured pins (implements common.ButtonDriver interface)
func (bd *ButtonDriver) Start() error {
	bd.mutex.Lock()
	defer bd.mutex.Unlock()

	if bd.started {
		return fmt.Errorf("driver already started")
	}

	if len(bd.pins) == 0 {
		return fmt.Errorf("no buttons configured")
	}

	bd.started = true
	for _, buttonPin := range bd.pins {
		bd.wg.Add(1)
		go bd.monitorPin(buttonPin)
	}

	log.Printf("Started monitoring %d GPIO buttons", len(bd.pins))
	return nil
}

// Stop stops monitoring all pins (implements common.ButtonDriver interface)
func (bd *ButtonDriver) Stop() {
	bd.mutex.Lock()
	defer bd.mutex.Unlock()

	if !bd.started {
		return
	}

	close(bd.stopChannel)
	bd.wg.Wait()
	close(bd.eventChannel)
	bd.started = false
	log.Printf("Stopped GPIO button monitoring")
}

// Events returns the channel for receiving button events (implements common.ButtonDriver interface)
func (bd *ButtonDriver) Events() <-chan common.ButtonEvent {
	return bd.eventChannel
}

// GetButtons returns a list of button sources being monitored (implements common.ButtonDriver interface)
func (bd *ButtonDriver) GetButtons() []string {
	bd.mutex.RLock()
	defer bd.mutex.RUnlock()

	buttons := make([]string, 0, len(bd.pins))
	for name := range bd.pins {
		buttons = append(buttons, name)
	}
	return buttons
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
	currentState := bd.readButtonState(buttonPin.pin, buttonPin.polarity)
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

			// Send event using common.ButtonEvent
			eventType := common.ButtonReleased
			if currentState {
				eventType = common.ButtonPressed
			}

			event := common.ButtonEvent{
				Source:    buttonPin.name,
				Type:      eventType,
				Timestamp: now,
				Device:    buttonPin.pinName,
				Metadata: map[string]interface{}{
					"gpio_pin": buttonPin.pinName,
					"pressed":  currentState,
				},
			}

			select {
			case bd.eventChannel <- event:
			default:
				log.Printf("Warning: event channel full, dropping event for button %s", buttonPin.name)
			}
		} else {
			// State is the same as last reported, just mark as reported
			buttonPin.stateReported = true
		}
	}
}

// readButtonState reads the current logical state of a button pin
func (bd *ButtonDriver) readButtonState(pin gpio.PinIO, polarity gpio.Level) bool {
	level := pin.Read()
	return level == polarity
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
func (bd *ButtonDriver) getPullResistor(pullMode PullMode, activeHigh bool) gpio.Pull {
	switch pullMode {
	case PullUp:
		return gpio.PullUp
	case PullDown:
		return gpio.PullDown
	case PullAuto:
		// Auto mode: choose pull resistor based on polarity
		if activeHigh {
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
func (bd *ButtonDriver) getPullString(pullMode PullMode, activeHigh bool) string {
	switch pullMode {
	case PullUp:
		return "up"
	case PullDown:
		return "down"
	case PullAuto:
		if activeHigh {
			return "down (auto)"
		}
		return "up (auto)"
	case PullNone:
		fallthrough
	default:
		return "none"
	}
}

// GetPullMode returns the current default pull resistor mode
func (bd *ButtonDriver) GetPullMode() PullMode {
	return bd.defaultPullMode
}

// Ensure ButtonDriver implements the common.ButtonDriver interface
var _ common.ButtonDriver = (*ButtonDriver)(nil)
