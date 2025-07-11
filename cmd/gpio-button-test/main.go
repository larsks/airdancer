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

	"github.com/larsks/airdancer/internal/buttondriver/gpio"
)

func main() {
	var (
		pins         = flag.String("pins", "", "Comma-separated list of GPIO pin names (e.g., GPIO18,GPIO19)")
		debounceMs   = flag.Int("debounce", 50, "Debounce delay in milliseconds")
		activeHigh   = flag.Bool("active-high", true, "Whether buttons are active-high (default: true)")
		pullMode     = flag.String("pull", "auto", "Pull resistor mode: none, up, down, auto (default: auto)")
		showHelp     = flag.Bool("help", false, "Show help message")
	)
	flag.Parse()

	if *showHelp {
		fmt.Println("GPIO Button Test Program")
		fmt.Println("========================")
		fmt.Println()
		fmt.Println("This program monitors GPIO pins for button press/release events with debouncing.")
		fmt.Println()
		fmt.Println("Usage:")
		flag.PrintDefaults()
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  # Monitor GPIO18 and GPIO19 with 50ms debounce (active-high, auto pull)")
		fmt.Println("  gpio-button-test -pins=GPIO18,GPIO19")
		fmt.Println()
		fmt.Println("  # Monitor GPIO18 with 100ms debounce (active-low, manual pull-up)")
		fmt.Println("  gpio-button-test -pins=GPIO18 -debounce=100 -active-high=false -pull=up")
		fmt.Println()
		fmt.Println("  # Monitor GPIO19 with no pull resistor")
		fmt.Println("  gpio-button-test -pins=GPIO19 -pull=none")
		fmt.Println()
		fmt.Println("Pull modes:")
		fmt.Println("  none - No pull resistor")
		fmt.Println("  up   - Enable pull-up resistor")
		fmt.Println("  down - Enable pull-down resistor")
		fmt.Println("  auto - Automatically choose based on active level (default)")
		fmt.Println()
		fmt.Println("Press Ctrl+C to stop monitoring.")
		return
	}

	if *pins == "" {
		log.Fatal("Error: -pins parameter is required. Use -help for usage information.")
	}

	// Parse pin names
	pinNames := strings.Split(*pins, ",")
	for i, pin := range pinNames {
		pinNames[i] = strings.TrimSpace(pin)
	}

	// Parse pull mode
	var pullModeEnum gpio.PullMode
	switch strings.ToLower(*pullMode) {
	case "none":
		pullModeEnum = gpio.PullNone
	case "up":
		pullModeEnum = gpio.PullUp
	case "down":
		pullModeEnum = gpio.PullDown
	case "auto":
		pullModeEnum = gpio.PullAuto
	default:
		log.Fatalf("Invalid pull mode: %s. Valid options: none, up, down, auto", *pullMode)
	}

	// Create button driver
	debounceDelay := time.Duration(*debounceMs) * time.Millisecond
	driver, err := gpio.NewButtonDriver(debounceDelay, *activeHigh, pullModeEnum)
	if err != nil {
		log.Fatalf("Failed to create button driver: %v", err)
	}

	// Add pins
	for _, pinName := range pinNames {
		if err := driver.AddPin(pinName); err != nil {
			log.Fatalf("Failed to add pin %s: %v", pinName, err)
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
	fmt.Println("GPIO Button Monitor")
	fmt.Println("===================")
	fmt.Printf("Monitoring pins: %v\n", pinNames)
	fmt.Printf("Debounce delay: %v\n", debounceDelay)
	fmt.Printf("Active level: %s\n", map[bool]string{true: "HIGH", false: "LOW"}[*activeHigh])
	fmt.Printf("Pull resistor: %s\n", *pullMode)
	fmt.Println()
	fmt.Println("Press Ctrl+C to stop...")
	fmt.Println()

	// Monitor events
	go func() {
		for event := range driver.Events() {
			timestamp := event.Timestamp.Format("15:04:05.000")
			state := map[bool]string{true: "PRESSED", false: "RELEASED"}[event.Pressed]
			fmt.Printf("[%s] Pin %s: %s\n", timestamp, event.Pin, state)
		}
	}()

	// Wait for signal
	<-signalChan
	fmt.Println("\nShutting down...")
	driver.Stop()
	fmt.Println("Goodbye!")
}