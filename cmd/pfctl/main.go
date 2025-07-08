package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/larsks/airdancer/internal/piface"
	"github.com/larsks/airdancer/internal/version"
	"github.com/spf13/pflag"
)

var (
	versionFlag = pflag.Bool("version", false, "Show version and exit")
	spiDevice   = pflag.String("spi-device", "/dev/spidev0.0", "SPI device path")
	helpFlag    = pflag.BoolP("help", "h", false, "Show help")
	duration    = pflag.Duration("duration", 0, "Duration to run blink command (0 = run indefinitely)")
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [OPTIONS] COMMAND [ARGS...]\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "A command line tool for controlling PiFace Digital I/O boards.\n\n")

	fmt.Fprintf(os.Stderr, "Commands:\n")
	fmt.Fprintf(os.Stderr, "  read inputs     Read current input pin states\n")
	fmt.Fprintf(os.Stderr, "  read outputs    Read current output pin states\n")
	fmt.Fprintf(os.Stderr, "  write pin:value Set output pins to specified values\n")
	fmt.Fprintf(os.Stderr, "  reflect         Continuously mirror input pins to output pins\n")
	fmt.Fprintf(os.Stderr, "  blink pin:period[:dutycycle] Blink output pins with specified period and duty cycle\n\n")

	fmt.Fprintf(os.Stderr, "Options:\n")
	pflag.PrintDefaults()

	fmt.Fprintf(os.Stderr, "\nExamples:\n")
	fmt.Fprintf(os.Stderr, "  %s read inputs              # Read all input pins\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s read outputs             # Read all output pins\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s write 0:1 1:0 2:1        # Set pin 0 on, pin 1 off, pin 2 on\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s write 0:on 1:off         # Alternative syntax with on/off\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s reflect                  # Mirror inputs to outputs continuously\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s blink 0:1s 1:500ms:0.3   # Blink pin 0 every 1s, pin 1 every 500ms at 30%% duty cycle\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s --duration 10s blink 0:2s # Blink pin 0 every 2s for 10 seconds\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s --spi-device /dev/spidev0.1 read inputs  # Use alternative SPI device\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\nPin values for write command:\n")
	fmt.Fprintf(os.Stderr, "  on, 1, true     Turn pin on\n")
	fmt.Fprintf(os.Stderr, "  off, 0, false   Turn pin off\n")
}

func main() {
	pflag.Parse()

	if *versionFlag {
		version.ShowVersion()
		os.Exit(0)
	}

	if *helpFlag {
		usage()
		os.Exit(0)
	}

	args := pflag.Args()
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Error: No command specified\n\n")
		usage()
		os.Exit(1)
	}

	command := args[0]

	// Initialize PiFace
	pf, err := piface.NewPiFace(false, *spiDevice, 0)
	if err != nil {
		log.Fatalf("Failed to initialize PiFace on %s: %v", *spiDevice, err)
	}
	defer pf.Close() //nolint:errcheck

	if err := pf.Init(); err != nil {
		log.Fatalf("Failed to initialize PiFace: %v", err)
	}

	switch command {
	case "read":
		if err := handleReadCommand(pf, args[1:]); err != nil {
			log.Fatalf("Read command failed: %v", err)
		}
	case "write":
		if err := handleWriteCommand(pf, args[1:]); err != nil {
			log.Fatalf("Write command failed: %v", err)
		}
	case "reflect":
		if err := handleReflectCommand(pf, args[1:]); err != nil {
			log.Fatalf("Reflect command failed: %v", err)
		}
	case "blink":
		if err := handleBlinkCommand(pf, args[1:]); err != nil {
			log.Fatalf("Blink command failed: %v", err)
		}
	default:
		fmt.Fprintf(os.Stderr, "Error: Unknown command '%s'\n\n", command)
		usage()
		os.Exit(1)
	}
}

func handleReadCommand(pf *piface.PiFace, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("read command requires 'inputs' or 'outputs' argument")
	}

	target := args[0]
	switch target {
	case "inputs":
		return readInputs(pf)
	case "outputs":
		return readOutputs(pf)
	default:
		return fmt.Errorf("invalid read target '%s': must be 'inputs' or 'outputs'", target)
	}
}

func handleWriteCommand(pf *piface.PiFace, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("write command requires at least one pin:value pair")
	}

	for _, arg := range args {
		parts := strings.SplitN(arg, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid pin:value format: %s", arg)
		}

		pinStr := parts[0]
		valueStr := parts[1]

		pin, err := strconv.ParseUint(pinStr, 10, 8)
		if err != nil {
			return fmt.Errorf("invalid pin number '%s': %v", pinStr, err)
		}

		if pin > 7 {
			return fmt.Errorf("invalid pin number %d: must be 0-7", pin)
		}

		value, err := parseValue(valueStr)
		if err != nil {
			return fmt.Errorf("invalid value '%s' for pin %d: %v", valueStr, pin, err)
		}

		if err := pf.WriteOutput(uint8(pin), value); err != nil {
			return fmt.Errorf("failed to write pin %d: %v", pin, err)
		}

		fmt.Printf("Set pin %d to %d\n", pin, value)
	}

	return nil
}

func handleReflectCommand(pf *piface.PiFace, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("reflect command does not accept arguments")
	}

	fmt.Println("Starting input-to-output reflection. Press Ctrl+C to stop.")
	fmt.Println("Input changes will be displayed and mirrored to outputs.")

	// Reflect inputs to outputs
	var oldval uint8
	for {
		val, err := pf.ReadInputs()
		if err != nil {
			return fmt.Errorf("failed to read inputs: %v", err)
		}

		if val != oldval {
			fmt.Printf("INPUTS: ")
			for i := range 8 {
				fmt.Printf("%d ", (val>>(7-i))&0x1)
			}
			fmt.Println("")
			oldval = val
		}

		if err := pf.WriteOutputs(val); err != nil {
			return fmt.Errorf("failed to write outputs: %v", err)
		}

		time.Sleep(10 * time.Millisecond)
	}
}

func readInputs(pf *piface.PiFace) error {
	inputs, err := pf.ReadInputs()
	if err != nil {
		return err
	}

	fmt.Printf("Input pins (7-0): ")
	for i := 7; i >= 0; i-- {
		value := (inputs >> i) & 1
		fmt.Printf("%d", value)
	}
	fmt.Println()

	// Also show individual pin states
	fmt.Println("Individual input pins:")
	for i := 0; i < 8; i++ {
		value := (inputs >> i) & 1
		fmt.Printf("  Pin %d: %d\n", i, value)
	}

	return nil
}

func readOutputs(pf *piface.PiFace) error {
	outputs, err := pf.ReadOutputs()
	if err != nil {
		return err
	}

	fmt.Printf("Output pins (7-0): ")
	for i := 7; i >= 0; i-- {
		value := (outputs >> i) & 1
		fmt.Printf("%d", value)
	}
	fmt.Println()

	// Also show individual pin states
	fmt.Println("Individual output pins:")
	for i := 0; i < 8; i++ {
		value := (outputs >> i) & 1
		fmt.Printf("  Pin %d: %d\n", i, value)
	}

	return nil
}

type blinkConfig struct {
	pin       uint8
	period    time.Duration
	dutyCycle float64
}

func handleBlinkCommand(pf *piface.PiFace, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("blink command requires at least one pin:period[:dutycycle] argument")
	}

	var configs []blinkConfig
	for _, arg := range args {
		config, err := parseBlinkArg(arg)
		if err != nil {
			return fmt.Errorf("invalid blink argument '%s': %v", arg, err)
		}
		configs = append(configs, config)
	}

	if *duration > 0 {
		fmt.Printf("Starting blink for %v\n", *duration)
	} else {
		fmt.Println("Starting blink indefinitely. Press Ctrl+C to stop.")
	}

	var endTime time.Time
	if *duration > 0 {
		endTime = time.Now().Add(*duration)
	}

	for _, config := range configs {
		go blinkPin(pf, config, endTime)
	}

	if *duration > 0 {
		time.Sleep(*duration)
	} else {
		select {}
	}

	return nil
}

func parseBlinkArg(arg string) (blinkConfig, error) {
	parts := strings.Split(arg, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return blinkConfig{}, fmt.Errorf("format must be pin:period[:dutycycle]")
	}

	pin, err := strconv.ParseUint(parts[0], 10, 8)
	if err != nil {
		return blinkConfig{}, fmt.Errorf("invalid pin number '%s': %v", parts[0], err)
	}
	if pin > 7 {
		return blinkConfig{}, fmt.Errorf("invalid pin number %d: must be 0-7", pin)
	}

	period, err := time.ParseDuration(parts[1])
	if err != nil {
		return blinkConfig{}, fmt.Errorf("invalid period '%s': %v", parts[1], err)
	}
	if period <= 0 {
		return blinkConfig{}, fmt.Errorf("period must be positive")
	}

	dutyCycle := 0.5
	if len(parts) == 3 {
		dutyCycle, err = strconv.ParseFloat(parts[2], 64)
		if err != nil {
			return blinkConfig{}, fmt.Errorf("invalid duty cycle '%s': %v", parts[2], err)
		}
		if dutyCycle < 0 || dutyCycle > 1 {
			return blinkConfig{}, fmt.Errorf("duty cycle must be between 0 and 1")
		}
	}

	return blinkConfig{
		pin:       uint8(pin),
		period:    period,
		dutyCycle: dutyCycle,
	}, nil
}

func blinkPin(pf *piface.PiFace, config blinkConfig, endTime time.Time) {
	onTime := time.Duration(float64(config.period) * config.dutyCycle)
	offTime := config.period - onTime

	for {
		if !endTime.IsZero() && time.Now().After(endTime) {
			pf.WriteOutput(config.pin, 0) //nolint:errcheck
			return
		}

		if err := pf.WriteOutput(config.pin, 1); err != nil {
			log.Printf("failed to set pin %d", config.pin)
			return
		}
		time.Sleep(onTime)

		if !endTime.IsZero() && time.Now().After(endTime) {
			pf.WriteOutput(config.pin, 0) //nolint:errcheck
			return
		}

		if err := pf.WriteOutput(config.pin, 0); err != nil {
			log.Printf("failed to reset pin %d", config.pin)
			return
		}
		time.Sleep(offTime)
	}
}

func parseValue(valueStr string) (uint8, error) {
	switch strings.ToLower(valueStr) {
	case "1", "on", "true":
		return 1, nil
	case "0", "off", "false":
		return 0, nil
	default:
		return 0, fmt.Errorf("invalid value (must be 0/1, on/off, or true/false)")
	}
}
