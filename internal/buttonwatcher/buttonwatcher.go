package buttonwatcher

import (
	"cmp"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/larsks/airdancer/internal/buttondriver"
	"github.com/larsks/airdancer/internal/buttondriver/common"

	// Import button driver factories to register them
	_ "github.com/larsks/airdancer/internal/buttondriver"
)

// ButtonWrapper wraps a button driver with action handling functionality
type ButtonWrapper struct {
	name               string
	driver             common.ButtonDriver
	clickAction        string
	doubleClickAction  string
	tripleClickAction  string
	clickInterval      time.Duration
	shortPressDuration time.Duration
	shortPressAction   string
	longPressDuration  time.Duration
	longPressAction    string
	timeout            time.Duration
	defaultAction      string

	// State tracking for click/press detection
	clickCount     int
	lastClickTime  time.Time
	clickTimer     *time.Timer
	pressStartTime time.Time
	isPressed      bool
	mutex          sync.Mutex
}

type ButtonMonitor struct {
	drivers      map[string]common.ButtonDriver
	wrappers     []*ButtonWrapper
	stopChan     chan struct{}
	wg           sync.WaitGroup
	debounceMs   int
	pullMode     string
	globalConfig *Config
}

func NewButtonMonitor() *ButtonMonitor {
	return &ButtonMonitor{
		drivers:    make(map[string]common.ButtonDriver),
		wrappers:   make([]*ButtonWrapper, 0),
		stopChan:   make(chan struct{}),
		debounceMs: 50,
		pullMode:   "auto",
	}
}

// SetGlobalConfig sets the global configuration for default values
func (bm *ButtonMonitor) SetGlobalConfig(config *Config) {
	bm.globalConfig = config
}

func (bm *ButtonMonitor) createDriver(driverType string) (common.ButtonDriver, error) {
	// Prepare driver-specific configuration
	config := make(map[string]interface{})

	// Add GPIO-specific configuration if this is a GPIO driver
	if driverType == "gpio" {
		config["pull-mode"] = bm.pullMode
		config["debounce-ms"] = bm.debounceMs
	}

	return buttondriver.CreateDriver(driverType, config)
}

func (bm *ButtonMonitor) AddButtonFromConfig(config ButtonConfig) error {
	// Get or create driver for this type
	driverType := config.Driver
	driver, exists := bm.drivers[driverType]
	if !exists {
		var err error
		driver, err = bm.createDriver(driverType)
		if err != nil {
			return fmt.Errorf("failed to create %s driver: %v", driverType, err)
		}
		bm.drivers[driverType] = driver
	}

	// Parse the button spec and add to driver
	fullSpec := config.Name + ":" + config.Spec
	buttonSpec, err := buttondriver.ParseButtonSpec(driverType, fullSpec)
	if err != nil {
		return fmt.Errorf("failed to parse button spec for %s: %w", config.Name, err)
	}

	if err := driver.AddButton(buttonSpec); err != nil {
		return fmt.Errorf("failed to add button %s to driver: %v", config.Name, err)
	}

	// Create wrapper for action handling
	wrapper := &ButtonWrapper{
		name:   config.Name,
		driver: driver,
	}

	// Set action configuration
	if config.ClickAction != nil {
		wrapper.clickAction = *config.ClickAction
	}
	if config.DoubleClickAction != nil {
		wrapper.doubleClickAction = *config.DoubleClickAction
	}
	if config.TripleClickAction != nil {
		wrapper.tripleClickAction = *config.TripleClickAction
	}

	// Set timing configuration with global defaults
	wrapper.clickInterval = bm.getClickInterval(config.ClickInterval)
	wrapper.shortPressDuration = bm.getShortPressDuration(config.ShortPressDuration)
	wrapper.longPressDuration = bm.getLongPressDuration(config.LongPressDuration)
	wrapper.timeout = bm.getTimeout(config.Timeout)

	if config.ShortPressAction != nil {
		wrapper.shortPressAction = *config.ShortPressAction
	}
	if config.LongPressAction != nil {
		wrapper.longPressAction = *config.LongPressAction
	}
	// Set default action with fallback to global default
	if config.DefaultAction != nil {
		wrapper.defaultAction = *config.DefaultAction
	} else if bm.globalConfig != nil && bm.globalConfig.DefaultAction != nil {
		wrapper.defaultAction = *bm.globalConfig.DefaultAction
	}

	bm.wrappers = append(bm.wrappers, wrapper)
	return nil
}

// getClickInterval returns the click interval, using button-specific value or global default
func (bm *ButtonMonitor) getClickInterval(buttonValue *time.Duration) time.Duration {
	if buttonValue != nil {
		return *buttonValue
	}
	if bm.globalConfig != nil && bm.globalConfig.ClickInterval != nil {
		return *bm.globalConfig.ClickInterval
	}
	return 500 * time.Millisecond // default value
}

// getShortPressDuration returns the short press duration, using button-specific value or global default
func (bm *ButtonMonitor) getShortPressDuration(buttonValue *time.Duration) time.Duration {
	if buttonValue != nil {
		return *buttonValue
	}
	if bm.globalConfig != nil && bm.globalConfig.ShortPressDuration != nil {
		return *bm.globalConfig.ShortPressDuration
	}
	return 0 // default value (disabled)
}

// getLongPressDuration returns the long press duration, using button-specific value or global default
func (bm *ButtonMonitor) getLongPressDuration(buttonValue *time.Duration) time.Duration {
	if buttonValue != nil {
		return *buttonValue
	}
	if bm.globalConfig != nil && bm.globalConfig.LongPressDuration != nil {
		return *bm.globalConfig.LongPressDuration
	}
	return 0 // default value (disabled)
}

// getTimeout returns the timeout, using button-specific value or global default
func (bm *ButtonMonitor) getTimeout(buttonValue *time.Duration) time.Duration {
	if buttonValue != nil {
		return *buttonValue
	}
	if bm.globalConfig != nil && bm.globalConfig.Timeout != nil {
		return *bm.globalConfig.Timeout
	}
	return 10 * time.Second // default value
}

func (bm *ButtonMonitor) Start() error {
	if len(bm.drivers) == 0 {
		return fmt.Errorf("no button drivers configured")
	}

	// Start all drivers
	for driverType, driver := range bm.drivers {
		if err := driver.Start(); err != nil {
			return fmt.Errorf("failed to start %s driver: %v", driverType, err)
		}
		log.Printf("Started %s button driver", driverType)

		// Start event processor for this driver
		bm.wg.Add(1)
		go bm.processEvents(driver)
	}

	select {}
}

func (bm *ButtonMonitor) processEvents(driver common.ButtonDriver) {
	defer bm.wg.Done()

	for {
		select {
		case <-bm.stopChan:
			return
		case event, ok := <-driver.Events():
			if !ok {
				return
			}
			bm.handleButtonEvent(event)
		}
	}
}

func (bm *ButtonMonitor) handleButtonEvent(event common.ButtonEvent) {
	// Find the wrapper for this button
	var wrapper *ButtonWrapper
	for _, w := range bm.wrappers {
		if w.name == event.Source {
			wrapper = w
			break
		}
	}

	if wrapper == nil {
		log.Printf("No wrapper found for button: %s", event.Source)
		return
	}

	wrapper.mutex.Lock()
	defer wrapper.mutex.Unlock()

	switch event.Type {
	case common.ButtonPressed:
		wrapper.isPressed = true
		wrapper.pressStartTime = event.Timestamp
		log.Printf("[%s] Button PRESSED", event.Source)

	case common.ButtonReleased:
		if wrapper.isPressed {
			wrapper.isPressed = false
			holdDuration := event.Timestamp.Sub(wrapper.pressStartTime)

			log.Printf("[%s] Button RELEASED - Hold duration: %.2f seconds", event.Source, holdDuration.Seconds())

			// Determine action based on hold duration
			if wrapper.timeout > 0 && holdDuration >= wrapper.timeout {
				log.Printf("[%s] Hold duration >= timeout (%.1fs): No action taken", event.Source, wrapper.timeout.Seconds())
			} else if wrapper.longPressDuration > 0 && holdDuration >= wrapper.longPressDuration {
				action := cmp.Or(wrapper.longPressAction, wrapper.defaultAction)
				log.Printf("[%s] Long press detected (%.1fs): %s", event.Source, wrapper.longPressDuration.Seconds(), action)
				wrapper.executeCommand(action, "long-press")
			} else if wrapper.shortPressDuration > 0 && holdDuration >= wrapper.shortPressDuration {
				action := cmp.Or(wrapper.shortPressAction, wrapper.defaultAction)
				log.Printf("[%s] Short press detected (%.1fs): %s", event.Source, wrapper.shortPressDuration.Seconds(), action)
				wrapper.executeCommand(action, "short-press")
			} else {
				// Handle click sequence
				wrapper.handleClickSequence(event.Timestamp)
			}
		}
	}
}

func (wrapper *ButtonWrapper) handleClickSequence(releaseTime time.Time) {
	// Check if this is within the click interval from the last click
	if !wrapper.lastClickTime.IsZero() && releaseTime.Sub(wrapper.lastClickTime) <= wrapper.clickInterval {
		wrapper.clickCount++
	} else {
		wrapper.clickCount = 1
	}

	wrapper.lastClickTime = releaseTime

	// Cancel any existing click timer
	if wrapper.clickTimer != nil {
		wrapper.clickTimer.Stop()
	}

	// Start a new timer to wait for potential additional clicks
	wrapper.clickTimer = time.AfterFunc(wrapper.clickInterval, func() {
		wrapper.mutex.Lock()
		clickCount := wrapper.clickCount
		wrapper.clickCount = 0
		wrapper.lastClickTime = time.Time{}
		wrapper.mutex.Unlock()

		wrapper.executeClickAction(clickCount)
	})
}

func (wrapper *ButtonWrapper) executeClickAction(clickCount int) {
	var action string
	var actionType string

	switch clickCount {
	case 1:
		action = cmp.Or(wrapper.clickAction, wrapper.defaultAction)
		actionType = "single-click"
	case 2:
		action = cmp.Or(wrapper.doubleClickAction, wrapper.defaultAction)
		actionType = "double-click"
	case 3:
		action = cmp.Or(wrapper.tripleClickAction, wrapper.defaultAction)
		actionType = "triple-click"
	default:
		return
	}

	if action != "" {
		log.Printf("[%s] %s detected: %s", wrapper.name, actionType, action)
		wrapper.executeCommand(action, actionType)
	}
}

func (wrapper *ButtonWrapper) executeCommand(command string, actionType string) {
	if command == "" {
		log.Printf("[%s] No command", wrapper.name)
		return
	}
	log.Printf("[%s] Executing command: %s", wrapper.name, command)
	cmd := exec.Command("sh", "-c", command)
	env := os.Environ()
	env = append(env, fmt.Sprintf("BUTTON_ACTION_TYPE=%s", actionType))
	env = append(env, fmt.Sprintf("BUTTON_NAME=%s", wrapper.name))
	cmd.Env = env
	if err := cmd.Run(); err != nil {
		log.Printf("[%s] Error executing command: %v", wrapper.name, err)
	}
}

func (bm *ButtonMonitor) Close() error {
	close(bm.stopChan)
	bm.wg.Wait()

	for driverType, driver := range bm.drivers {
		driver.Stop()
		log.Printf("Stopped %s button driver", driverType)
	}

	return nil
}
