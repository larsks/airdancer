package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/larsks/airdancer/internal/buttondriver/common"
	"github.com/larsks/airdancer/internal/buttondriver/event"
	"github.com/larsks/airdancer/internal/buttondriver/gpio"
)

func main() {
	var (
		buttons      = flag.String("buttons", "", "Comma-separated list of button specs (format: driver:name:spec)")
		debounceMs   = flag.Int("debounce", 50, "Debounce delay in milliseconds (GPIO only)")
		pullMode     = flag.String("pull", "auto", "Default pull resistor mode: none, up, down, auto (GPIO only)")
		showHelp     = flag.Bool("help", false, "Show help message")
	)
	flag.Parse()

	if *showHelp {
		fmt.Println("Common Button Interface Test Program")
		fmt.Println("====================================")
		fmt.Println()
		fmt.Println("This program demonstrates the common button interface that can be used")
		fmt.Println("with different button implementations (GPIO, USB HID, etc.).")
		fmt.Println()
		fmt.Println("Usage:")
		flag.PrintDefaults()
		fmt.Println()
		fmt.Println("Button Specification Format:")
		fmt.Println("  driver:name:spec")
		fmt.Println()
		fmt.Println("GPIO Driver Format:")
		fmt.Println("  gpio:name:pin[:active-high|active-low][:pull-none|pull-up|pull-down|pull-auto]")
		fmt.Println()
		fmt.Println("Event Driver Format:")
		fmt.Println("  event:name:device:event_type:event_code[:low_value:high_value]")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  # Simple GPIO button on pin 16")
		fmt.Println("  common-button-test -buttons=gpio:btn1:GPIO16")
		fmt.Println()
		fmt.Println("  # Multiple buttons with different drivers")
		fmt.Println("  common-button-test -buttons=\"gpio:btn1:GPIO16:active-low:pull-up,event:power:/dev/input/event0:EV_KEY:116\"")
		fmt.Println()
		fmt.Println("  # Event button with custom values")
		fmt.Println("  common-button-test -buttons=event:volume_up:/dev/input/event1:EV_KEY:115:0:1")
		fmt.Println()
		fmt.Println("  # Button with custom debounce (GPIO only)")
		fmt.Println("  common-button-test -buttons=gpio:btn1:GPIO16 -debounce=100")
		fmt.Println()
		fmt.Println("Driver Types:")
		fmt.Println("  gpio  - GPIO-based buttons")
		fmt.Println("  event - Input event device buttons")
		fmt.Println()
		fmt.Println("Press Ctrl+C to stop monitoring.")
		return
	}

	if *buttons == "" {
		log.Fatal("Error: -buttons parameter is required. Use -help for usage information.")
	}

	// Parse and create drivers for each button spec
	buttonSpecs := strings.Split(*buttons, ",")
	drivers := make(map[string]common.ButtonDriver)
	
	for _, spec := range buttonSpecs {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			continue
		}

		// Parse driver:name:spec format
		parts := strings.SplitN(spec, ":", 3)
		if len(parts) < 3 {
			log.Fatalf("Invalid button spec format: %s. Expected: driver:name:spec", spec)
		}

		driverType := parts[0]
		buttonName := parts[1]
		buttonSpec := parts[2]

		// Create driver if not exists
		if _, exists := drivers[driverType]; !exists {
			driver, err := createDriver(driverType, *debounceMs, *pullMode)
			if err != nil {
				log.Fatalf("Failed to create %s driver: %v", driverType, err)
			}
			drivers[driverType] = driver
		}

		// Add button to the appropriate driver
		if err := addButton(drivers[driverType], driverType, buttonName, buttonSpec); err != nil {
			log.Fatalf("Failed to add button %s: %v", spec, err)
		}
	}

	// Start all drivers
	for driverType, driver := range drivers {
		if err := driver.Start(); err != nil {
			log.Fatalf("Failed to start %s driver: %v", driverType, err)
		}
	}

	// Set up signal handling
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	// Display configuration
	fmt.Println("Common Button Interface Monitor")
	fmt.Println("===============================")
	for driverType, driver := range drivers {
		fmt.Printf("Driver %s: %v\n", driverType, driver.GetButtons())
	}
	fmt.Printf("Debounce delay: %dms (GPIO only)\n", *debounceMs)
	fmt.Printf("Default pull mode: %s (GPIO only)\n", *pullMode)
	fmt.Println()
	fmt.Println("Press Ctrl+C to stop...")
	fmt.Println()

	// Monitor events from all drivers
	eventChan := make(chan common.ButtonEvent, 100)
	for _, driver := range drivers {
		go func(d common.ButtonDriver) {
			for event := range d.Events() {
				eventChan <- event
			}
		}(driver)
	}

	go func() {
		for event := range eventChan {
			timestamp := event.Timestamp.Format("15:04:05.000")
			fmt.Printf("[%s] Button '%s' on %s: %s\n", timestamp, event.Source, event.Device, event.Type)
			
			// Show metadata if available
			if len(event.Metadata) > 0 {
				fmt.Printf("         Metadata: %v\n", event.Metadata)
			}
		}
	}()

	// Wait for signal
	<-signalChan
	fmt.Println("\nShutting down...")
	for _, driver := range drivers {
		driver.Stop()
	}
	fmt.Println("Goodbye!")
}

func createDriver(driverType string, debounceMs int, pullMode string) (common.ButtonDriver, error) {
	switch driverType {
	case "gpio":
		return createGPIODriver(debounceMs, pullMode)
	case "event":
		return event.NewEventButtonDriver(), nil
	default:
		return nil, fmt.Errorf("unsupported driver type: %s", driverType)
	}
}

func createGPIODriver(debounceMs int, pullMode string) (common.ButtonDriver, error) {
	// Parse pull mode
	var pullModeEnum gpio.PullMode
	switch strings.ToLower(pullMode) {
	case "none":
		pullModeEnum = gpio.PullNone
	case "up":
		pullModeEnum = gpio.PullUp
	case "down":
		pullModeEnum = gpio.PullDown
	case "auto":
		pullModeEnum = gpio.PullAuto
	default:
		return nil, fmt.Errorf("invalid pull mode: %s. Valid options: none, up, down, auto", pullMode)
	}

	debounceDelay := time.Duration(debounceMs) * time.Millisecond
	return gpio.NewButtonDriver(debounceDelay, pullModeEnum)
}

func addButton(driver common.ButtonDriver, driverType string, buttonName string, buttonSpec string) error {
	switch driverType {
	case "gpio":
		// For GPIO, the spec format is: pin[:active-high|active-low][:pull-none|pull-up|pull-down|pull-auto]
		fullSpec := buttonName + ":" + buttonSpec
		gpioSpec, err := gpio.ParseGPIOButtonSpec(fullSpec)
		if err != nil {
			return fmt.Errorf("invalid GPIO button spec: %w", err)
		}
		return driver.AddButton(gpioSpec)
	case "event":
		// For event, the spec format is: device:event_type:event_code[:low_value:high_value]
		fullSpec := buttonName + ":" + buttonSpec
		eventSpec, err := event.ParseEventButtonSpec(fullSpec)
		if err != nil {
			return fmt.Errorf("invalid event button spec: %w", err)
		}
		return driver.AddButton(eventSpec)
	default:
		return fmt.Errorf("unsupported driver type: %s", driverType)
	}
}