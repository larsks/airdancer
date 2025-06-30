package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/larsks/airdancer/internal/gpiodriver"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: %s gpio_name:value [gpio_name:value...]\n", os.Args[0])
		os.Exit(1)
	}

	var pinNames []string
	actions := make(map[string]string)

	for _, arg := range os.Args[1:] {
		parts := strings.SplitN(arg, ":", 2)
		if len(parts) != 2 {
			log.Fatalf("invalid argument: %s", arg)
		}
		pinNames = append(pinNames, parts[0])
		actions[parts[0]] = parts[1]
	}

	collection, err := gpiodriver.NewGpioSwitchCollection(false, pinNames)
	if err != nil {
		log.Fatalf("failed to create switch collection: %s", err)
	}
	defer collection.Close()

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

