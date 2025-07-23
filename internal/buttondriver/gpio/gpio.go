package gpio

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/larsks/airdancer/internal/buttondriver/common"
	"github.com/larsks/airdancer/internal/gpio"
	"github.com/warthog618/go-gpiocdev"
)

// Legacy ButtonEvent type for backward compatibility
// Deprecated: Use common.ButtonEvent instead
type ButtonEvent struct {
	Pin       string
	Pressed   bool
	Timestamp time.Time
}

// ButtonDriver manages GPIO button monitoring with debouncing
type ButtonDriver struct {
	chip            *gpiocdev.Chip
	pins            map[string]*ButtonPin
	eventChannel    chan common.ButtonEvent
	stopChannel     chan struct{}
	wg              sync.WaitGroup
	debounceDelay   time.Duration
	defaultPullMode gpio.PullMode
	started         bool
	mutex           sync.RWMutex
}

// ButtonPin represents a single GPIO pin configured as a button
type ButtonPin struct {
	line          *gpiocdev.Line
	name          string
	pinName       string
	lastState     bool
	currentState  bool
	lastDebounce  time.Time
	stateReported bool
	polarity      int
	driver        *ButtonDriver
	mutex         sync.Mutex
}

// NewButtonDriver creates a new GPIO button driver
func NewButtonDriver(debounceDelay time.Duration, defaultPullMode gpio.PullMode) (*ButtonDriver, error) {
	// Open GPIO chip
	chip, err := gpiocdev.NewChip("gpiochip0")
	if err != nil {
		return nil, fmt.Errorf("failed to open GPIO chip: %w", err)
	}

	return &ButtonDriver{
		chip:            chip,
		pins:            make(map[string]*ButtonPin),
		eventChannel:    make(chan common.ButtonEvent, 100),
		stopChannel:     make(chan struct{}),
		debounceDelay:   debounceDelay,
		defaultPullMode: defaultPullMode,
	}, nil
}

// NewButtonDriverWithDefaults creates a new GPIO button driver with default settings
func NewButtonDriverWithDefaults() (*ButtonDriver, error) {
	return NewButtonDriver(50*time.Millisecond, gpio.PullAuto)
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

	// Parse pin number from spec.Pin (e.g., "GPIO16" -> 16)
	lineNum, err := gpio.ParsePinNumber(spec.Pin)
	if err != nil {
		return fmt.Errorf("invalid pin %s: %w", spec.Pin, err)
	}

	// Determine pull resistor configuration
	pullMode := spec.PullMode
	if pullMode == gpio.PullAuto {
		pullMode = bd.defaultPullMode
	}

	// Configure pin as input with pull resistor
	var lineOpts []gpiocdev.LineReqOption
	lineOpts = append(lineOpts, gpiocdev.AsInput)
	switch pullMode {
	case gpio.PullUp:
		lineOpts = append(lineOpts, gpiocdev.WithPullUp)
	case gpio.PullDown:
		lineOpts = append(lineOpts, gpiocdev.WithPullDown)
	case gpio.PullAuto:
		if spec.ActiveHigh {
			lineOpts = append(lineOpts, gpiocdev.WithPullDown)
		} else {
			lineOpts = append(lineOpts, gpiocdev.WithPullUp)
		}
	}

	line, err := bd.chip.RequestLine(lineNum, lineOpts...)
	if err != nil {
		return fmt.Errorf("failed to configure pin %s as input: %w", spec.Pin, err)
	}

	// Determine polarity
	polarity := 1
	if !spec.ActiveHigh {
		polarity = 0
	}

	initialState := bd.readButtonState(line, polarity)
	buttonPin := &ButtonPin{
		line:          line,
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

	// Close all GPIO lines
	for _, buttonPin := range bd.pins {
		if err := buttonPin.line.Close(); err != nil {
			log.Printf("Error closing GPIO line for button %s: %v", buttonPin.name, err)
		}
	}

	// Close the GPIO chip
	if bd.chip != nil {
		if err := bd.chip.Close(); err != nil {
			log.Printf("Error closing GPIO chip: %v", err)
		}
	}

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
	currentState := bd.readButtonState(buttonPin.line, buttonPin.polarity)
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
func (bd *ButtonDriver) readButtonState(line *gpiocdev.Line, polarity int) bool {
	level, err := line.Value()
	if err != nil {
		log.Printf("Error reading pin state: %v", err)
		return false
	}
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

// getPullString returns a human-readable string for the pull resistor configuration
func (bd *ButtonDriver) getPullString(pullMode gpio.PullMode, activeHigh bool) string {
	switch pullMode {
	case gpio.PullUp:
		return "up"
	case gpio.PullDown:
		return "down"
	case gpio.PullAuto:
		if activeHigh {
			return "down (auto)"
		}
		return "up (auto)"
	case gpio.PullNone:
		fallthrough
	default:
		return "none"
	}
}

// GetPullMode returns the current default pull resistor mode
func (bd *ButtonDriver) GetPullMode() gpio.PullMode {
	return bd.defaultPullMode
}

// Ensure ButtonDriver implements the common.ButtonDriver interface
var _ common.ButtonDriver = (*ButtonDriver)(nil)
