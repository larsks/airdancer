package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"
	"unsafe"

	"github.com/larsks/airdancer/internal/events"
)

// Button state for tracking individual button presses
type ButtonState struct {
	isPressed        bool
	pressTime        time.Time
	ticker           *time.Ticker
	stopTicker       chan bool
	lastSecond       int
	shortPressWarned bool
	longPressWarned  bool
	timeoutWarned    bool
	mutex            sync.Mutex
}

// Button monitoring configuration
type Button struct {
	Name               string
	Device             string
	EventType          events.EventType
	EventCode          uint32
	LowValue           uint32
	HighValue          uint32
	ClickAction        string
	ShortPressDuration time.Duration
	ShortPressAction   string
	LongPressDuration  time.Duration
	LongPressAction    string
	Timeout            time.Duration
	state              *ButtonState
}

type ButtonOption func(button *Button)

func NewButton(name string, device string, eventType events.EventType, eventCode uint32) *Button {
	return &Button{
		Name:      name,
		Device:    device,
		EventType: eventType,
		EventCode: eventCode,
		LowValue:  0,
		HighValue: 1,
		Timeout:   10 * time.Second,
		state: &ButtonState{
			mutex: sync.Mutex{},
		},
	}
}

func ShortPress(duration time.Duration, action string) ButtonOption {
	return func(button *Button) {
		button.ShortPressDuration = duration
		button.ShortPressAction = action
	}
}

func LongPress(duration time.Duration, action string) ButtonOption {
	return func(button *Button) {
		button.LongPressDuration = duration
		button.LongPressAction = action
	}
}

func Click(action string) ButtonOption {
	return func(button *Button) {
		button.ClickAction = action
	}
}

func Timeout(timeout time.Duration) ButtonOption {
	return func(button *Button) {
		button.Timeout = timeout
	}
}

func LowValue(val uint32) ButtonOption {
	return func(button *Button) {
		button.LowValue = val
	}
}

func HighValue(val uint32) ButtonOption {
	return func(button *Button) {
		button.HighValue = val
	}
}

func (button *Button) With(options ...ButtonOption) *Button {
	for _, option := range options {
		option(button)
	}
	return button
}

func (button *Button) executeCommand(command string) error {
	if command == "" {
		log.Printf("[%s] No command\n", button.Device)
		return nil
	}
	log.Printf("[%s] Executing command: %s\n", button.Device, command)
	cmd := exec.Command("sh", "-c", command)
	return cmd.Run()
}

func (button *Button) startHoldTimer() {
	button.state.mutex.Lock()

	// Check if timer is already running
	if button.state.ticker != nil {
		button.state.mutex.Unlock()
		return
	}

	// Initialize timer state
	button.state.ticker = time.NewTicker(time.Second)
	button.state.stopTicker = make(chan bool)
	button.state.lastSecond = 0
	button.state.shortPressWarned = false
	button.state.longPressWarned = false
	button.state.timeoutWarned = false

	button.state.mutex.Unlock() // Release mutex before starting goroutine

	go func() {
		for {
			select {
			case <-button.state.ticker.C:
				button.state.mutex.Lock()
				if button.state.isPressed {
					elapsed := time.Since(button.state.pressTime)
					seconds := int(elapsed.Seconds())

					if seconds > button.state.lastSecond {
						button.state.lastSecond = seconds
						//fmt.Printf("\r[%s] Button held for %d seconds", button.Device, seconds)

						// Check for threshold crossings
						if button.ShortPressDuration > 0 &&
							elapsed >= button.ShortPressDuration &&
							elapsed < button.LongPressDuration &&
							!button.state.shortPressWarned {
							log.Printf("Button %s - SHORT PRESS ZONE (%.0fs)", button.Name, button.ShortPressDuration.Seconds())
							button.state.shortPressWarned = true
						} else if button.LongPressDuration > 0 &&
							elapsed >= button.LongPressDuration &&
							elapsed < button.Timeout &&
							!button.state.longPressWarned {
							log.Printf("Button %s - LONG PRESS ZONE (%.0fs)", button.Name, button.LongPressDuration.Seconds())
							button.state.longPressWarned = true
						} else if button.Timeout > 0 &&
							elapsed >= button.Timeout &&
							!button.state.timeoutWarned {
							log.Printf("Button %s - TIMEOUT (no action)", button.Name)
							button.state.timeoutWarned = true
						}

						// Add dots for visual feedback
						if (button.ShortPressDuration > 0 && elapsed >= button.ShortPressDuration) ||
							(button.LongPressDuration > 0 && elapsed >= button.LongPressDuration) {
							fmt.Print("...")
						}
					}
				}
				button.state.mutex.Unlock()
			case <-button.state.stopTicker:
				return
			}
		}
	}()
}

func (button *Button) stopHoldTimer() {
	button.state.mutex.Lock()
	defer button.state.mutex.Unlock()

	if button.state.ticker != nil {
		button.state.ticker.Stop()
		button.state.stopTicker <- true
		close(button.state.stopTicker)
		button.state.ticker = nil
		log.Print() // New line after the counter
	}
}

func (button *Button) handleButtonPress() {
	button.state.mutex.Lock()
	defer button.state.mutex.Unlock()

	if !button.state.isPressed {
		button.state.isPressed = true
		button.state.pressTime = time.Now()
		log.Printf("[%s] Button %s PRESSED (value=%d)\n",
			button.Device, button.Name, button.HighValue)
	}

	// Start timer without holding the mutex
	button.state.mutex.Unlock()
	button.startHoldTimer()
	button.state.mutex.Lock()
}

func (button *Button) handleButtonRelease() {
	button.state.mutex.Lock()
	isPressed := button.state.isPressed
	pressTime := button.state.pressTime
	button.state.isPressed = false
	button.state.mutex.Unlock()

	if isPressed {
		releaseTime := time.Now()
		holdDuration := releaseTime.Sub(pressTime)

		button.stopHoldTimer()

		log.Printf("[%s] Button %s RELEASED (value=%d) - Hold duration: %.2f seconds\n",
			button.Device, button.Name, button.LowValue, holdDuration.Seconds())

		// Determine which action to execute based on hold duration
		if button.Timeout > 0 && holdDuration >= button.Timeout {
			log.Printf("[%s] Hold duration >= timeout (%.1fs): No action taken\n",
				button.Device, button.Timeout.Seconds())
		} else if button.LongPressDuration > 0 && holdDuration >= button.LongPressDuration {
			log.Printf("[%s] Long press detected (%.1fs): %s\n",
				button.Device, button.LongPressDuration.Seconds(), button.LongPressAction)
			if err := button.executeCommand(button.LongPressAction); err != nil {
				log.Printf("[%s] Error executing long press action: %v\n", button.Device, err)
			}
		} else if button.ShortPressDuration > 0 && holdDuration >= button.ShortPressDuration {
			log.Printf("[%s] Short press detected (%.1fs): %s\n",
				button.Device, button.ShortPressDuration.Seconds(), button.ShortPressAction)
			if err := button.executeCommand(button.ShortPressAction); err != nil {
				log.Printf("[%s] Error executing short press action: %v\n", button.Device, err)
			}
		} else {
			log.Printf("[%s] Click detected %s\n",
				button.Device, button.ClickAction)
			if err := button.executeCommand(button.ClickAction); err != nil {
				log.Printf("[%s] Error executing click action: %v\n", button.Device, err)
			}
		}
	}
}

func (button *Button) handleEvent(value uint32) {
	switch value {
	case button.HighValue:
		button.handleButtonPress()
	case button.LowValue:
		button.handleButtonRelease()
	default:
		// Ignore other values
	}
}

// ButtonMonitor manages multiple buttons across different devices
type ButtonMonitor struct {
	buttons map[string][]*Button // device -> buttons
	files   map[string]*os.File  // device -> file handle
}

func NewButtonMonitor() *ButtonMonitor {
	return &ButtonMonitor{
		buttons: make(map[string][]*Button),
		files:   make(map[string]*os.File),
	}
}

func (bm *ButtonMonitor) AddButton(button *Button) error {
	// Add button to the device's button list
	bm.buttons[button.Device] = append(bm.buttons[button.Device], button)

	// Open device file if not already open
	if _, exists := bm.files[button.Device]; !exists {
		file, err := os.Open(button.Device)
		if err != nil {
			return fmt.Errorf("error opening device %s: %v", button.Device, err)
		}
		bm.files[button.Device] = file
	}

	return nil
}

func (bm *ButtonMonitor) Start() error {
	if len(bm.buttons) == 0 {
		return fmt.Errorf("no buttons configured")
	}

	// Start monitoring each device in a separate goroutine
	for device, buttons := range bm.buttons {
		go bm.monitorDevice(device, buttons)
	}

	// Keep the main goroutine alive
	select {}
}

func (bm *ButtonMonitor) monitorDevice(device string, buttons []*Button) {
	file := bm.files[device]
	log.Printf("Monitoring device: %s with %d button(s)\n", device, len(buttons))

	eventSize := int(unsafe.Sizeof(events.InputEvent{}))
	buffer := make([]byte, eventSize)

	for {
		n, err := file.Read(buffer)
		if err != nil {
			log.Printf("Error reading from device %s: %v\n", device, err)
			break
		}

		if n != eventSize {
			log.Printf("Incomplete read from %s: got %d bytes, expected %d\n", device, n, eventSize)
			continue
		}

		// Parse the event
		var event events.InputEvent
		err = binary.Read(bytes.NewReader(buffer), binary.LittleEndian, &event)
		if err != nil {
			log.Printf("Error parsing event from %s: %v\n", device, err)
			continue
		}

		// Check if any button matches this event
		for _, button := range buttons {
			if events.EventType(event.Type) == button.EventType && uint32(event.Code) == button.EventCode {
				button.handleEvent(uint32(event.Value))
			}
		}
	}
}

func (bm *ButtonMonitor) Close() error {
	for device, file := range bm.files {
		if err := file.Close(); err != nil {
			return fmt.Errorf("failed to close device %v: %w", file, err)
		}
		log.Printf("Closed device: %s\n", device)
	}

	return nil
}

func main() {
	// Create button configurations
	powerButton := NewButton("power", "/dev/input/by-id/usb-Microntek_USB_Joystick-event-joystick", events.EV_KEY, 289).
		With(
			Click("xh -I localhost:8080/switch/0 state=on duration:=10"),
			ShortPress(2*time.Second, "sudo reboot"),
			LongPress(5*time.Second, "sudo poweroff"),
		)

	// You can add more buttons for different devices/events
	resetButton := NewButton("power", "/dev/input/by-id/usb-Microntek_USB_Joystick-event-joystick", events.EV_KEY, 288).
		With(
			Click("xh -I localhost:8080/switch/1 state=on duration:=10"),
			ShortPress(2*time.Second, "sudo systemctl start airdancer-wifi-fallback"),
		)

	// Create monitor and add buttons
	monitor := NewButtonMonitor()

	if err := monitor.AddButton(powerButton); err != nil {
		log.Fatalf("Error adding power button: %v\n", err)
	}

	if err := monitor.AddButton(resetButton); err != nil {
		log.Fatalf("Error adding reset button: %v\n", err)
	}

	defer monitor.Close() //nolint:errcheck

	log.Print("Starting button monitor...")
	if err := monitor.Start(); err != nil {
		log.Fatalf("Error starting monitor: %v\n", err)
	}
}
