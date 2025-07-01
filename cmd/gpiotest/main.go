package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/larsks/airdancer/internal/gpio"
)

func main() {
	var polarity string
	flag.StringVar(&polarity, "polarity", "ActiveHigh", "GPIO polarity: ActiveHigh or ActiveLow")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [--polarity ActiveHigh|ActiveLow] gpio_name:value [gpio_name:value...]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nFlags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s GPIO23:on GPIO24:off\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --polarity ActiveLow GPIO23:on\n", os.Args[0])
	}
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		flag.Usage()
		os.Exit(1)
	}

	// Validate polarity
	if polarity != "ActiveHigh" && polarity != "ActiveLow" {
		log.Fatalf("invalid polarity: %s (must be ActiveHigh or ActiveLow)", polarity)
	}

	var pinSpecs []string
	actions := make(map[string]string)

	for _, arg := range args {
		parts := strings.SplitN(arg, ":", 2)
		if len(parts) != 2 {
			log.Fatalf("invalid argument: %s", arg)
		}
		pinName := parts[0]
		pinSpec := fmt.Sprintf("%s:%s", pinName, polarity)
		pinSpecs = append(pinSpecs, pinSpec)
		actions[pinName] = parts[1]
	}

	collection, err := gpio.NewGPIOSwitchCollection(false, pinSpecs)
	if err != nil {
		log.Fatalf("failed to create switch collection: %s", err)
	}
	defer collection.Close() //nolint:errcheck

	if err := collection.Init(); err != nil {
		log.Fatalf("failed to initialize switch collection: %s", err)
	}

	switches := collection.ListSwitches()
	for _, s := range switches {
		pinName := s.String()
		action, ok := actions[pinName]
		if !ok {
			// This should not happen if logic is correct
			log.Printf("no action for pin %s", pinName)
			continue
		}

		switch strings.ToLower(action) {
		case "on", "1", "true":
			if err := s.TurnOn(); err != nil {
				log.Fatalf("failed to turn on %s: %s", pinName, err)
			}
		case "off", "0", "false":
			if err := s.TurnOff(); err != nil {
				log.Fatalf("failed to turn off %s: %s", pinName, err)
			}
		default:
			log.Fatalf("invalid value for %s: %s", pinName, action)
		}
	}
}
