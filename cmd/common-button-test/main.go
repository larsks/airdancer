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
	"github.com/larsks/airdancer/internal/buttondriver/gpio"
)

func main() {
	var (
		buttons      = flag.String("buttons", "", "Comma-separated list of button specs (format: name:pin[:active-high|active-low][:pull-none|pull-up|pull-down|pull-auto])")
		debounceMs   = flag.Int("debounce", 50, "Debounce delay in milliseconds")
		pullMode     = flag.String("pull", "auto", "Default pull resistor mode: none, up, down, auto")
		showHelp     = flag.Bool("help", false, "Show help message")
		driverType   = flag.String("driver", "gpio", "Button driver type: gpio (more drivers coming soon)")
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
		fmt.Println("  name:pin[:active-high|active-low][:pull-none|pull-up|pull-down|pull-auto]")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  # Simple GPIO button on pin 16")
		fmt.Println("  common-button-test -buttons=btn1:GPIO16")
		fmt.Println()
		fmt.Println("  # Multiple buttons with different configurations")
		fmt.Println("  common-button-test -buttons=\"btn1:GPIO16:active-low:pull-up,btn2:GPIO18:active-high:pull-down\"")
		fmt.Println()
		fmt.Println("  # Button with custom debounce")
		fmt.Println("  common-button-test -buttons=btn1:GPIO16 -debounce=100")
		fmt.Println()
		fmt.Println("Driver Types:")
		fmt.Println("  gpio - GPIO-based buttons (default)")
		fmt.Println("  hid  - USB HID buttons (coming soon)")
		fmt.Println()
		fmt.Println("Press Ctrl+C to stop monitoring.")
		return
	}

	if *buttons == "" {
		log.Fatal("Error: -buttons parameter is required. Use -help for usage information.")
	}

	// Create the appropriate driver
	var driver common.ButtonDriver
	var err error

	switch *driverType {
	case "gpio":
		driver, err = createGPIODriver(*debounceMs, *pullMode)
	default:
		log.Fatalf("Unsupported driver type: %s", *driverType)
	}

	if err != nil {
		log.Fatalf("Failed to create button driver: %v", err)
	}

	// Parse and add buttons
	buttonSpecs := strings.Split(*buttons, ",")
	for _, spec := range buttonSpecs {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			continue
		}

		if err := addButton(driver, spec, *driverType); err != nil {
			log.Fatalf("Failed to add button %s: %v", spec, err)
		}
	}

	// Start monitoring
	if err := driver.Start(); err != nil {
		log.Fatalf("Failed to start button driver: %v", err)
	}

	// Set up signal handling
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	// Display configuration
	fmt.Println("Common Button Interface Monitor")
	fmt.Println("===============================")
	fmt.Printf("Driver type: %s\n", *driverType)
	fmt.Printf("Buttons: %v\n", driver.GetButtons())
	fmt.Printf("Debounce delay: %dms\n", *debounceMs)
	if *driverType == "gpio" {
		fmt.Printf("Default pull mode: %s\n", *pullMode)
	}
	fmt.Println()
	fmt.Println("Press Ctrl+C to stop...")
	fmt.Println()

	// Monitor events
	go func() {
		for event := range driver.Events() {
			timestamp := event.Timestamp.Format("15:04:05.000")
			fmt.Printf("[%s] %s (%s): %s\n", timestamp, event.Source, event.Device, event.Type)
			
			// Show metadata if available
			if len(event.Metadata) > 0 {
				fmt.Printf("         Metadata: %v\n", event.Metadata)
			}
		}
	}()

	// Wait for signal
	<-signalChan
	fmt.Println("\nShutting down...")
	driver.Stop()
	fmt.Println("Goodbye!")
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

func addButton(driver common.ButtonDriver, spec string, driverType string) error {
	switch driverType {
	case "gpio":
		gpioSpec, err := gpio.ParseGPIOButtonSpec(spec)
		if err != nil {
			return fmt.Errorf("invalid GPIO button spec: %w", err)
		}
		return driver.AddButton(gpioSpec)
	default:
		return fmt.Errorf("unsupported driver type: %s", driverType)
	}
}