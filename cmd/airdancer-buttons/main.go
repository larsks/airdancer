package main

import (
	"log"
	"os"
	"time"

	"github.com/larsks/airdancer/internal/buttonwatcher"
	"github.com/larsks/airdancer/internal/events"
	"github.com/larsks/airdancer/internal/version"
	"github.com/spf13/pflag"
)

func main() {
	versionFlag := pflag.Bool("version", false, "Show version and exit")

	cfg := buttonwatcher.NewConfig()
	cfg.AddFlags(pflag.CommandLine)
	pflag.Parse()

	if *versionFlag {
		version.ShowVersion()
		os.Exit(0)
	}

	if err := cfg.LoadConfig(); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	monitor := buttonwatcher.NewButtonMonitor()
	defer monitor.Close() //nolint:errcheck

	for _, buttonCfg := range cfg.Buttons {
		eventType, ok := events.GetEventTypeName(buttonCfg.EventType)
		if !ok {
			log.Fatalf("button %s: unknown event type: %s", buttonCfg.Name, buttonCfg.EventType)
		}
		button := buttonwatcher.NewButton(buttonCfg.Name, buttonCfg.Device, eventType, buttonCfg.EventCode)

		if buttonCfg.Timeout != nil {
			button = button.With(buttonwatcher.Timeout(*buttonCfg.Timeout * time.Second))
		}

		if buttonCfg.ShortPressAction != nil {
			if buttonCfg.ShortPressDuration == nil {
				log.Fatalf("button %s: missing ShortPressDuration", buttonCfg.Name)
			}

			button = button.With(buttonwatcher.ShortPress(*buttonCfg.ShortPressDuration*time.Second, *buttonCfg.ShortPressAction))
		}

		if buttonCfg.LongPressAction != nil {
			if buttonCfg.LongPressDuration == nil {
				log.Fatalf("button %s: missing LongPressDuration", buttonCfg.Name)
			}

			button = button.With(buttonwatcher.LongPress(*buttonCfg.LongPressDuration*time.Second, *buttonCfg.LongPressAction))
		}

		if buttonCfg.ClickAction != nil {
			button = button.With(buttonwatcher.Click(*buttonCfg.ClickAction))
		}

		if err := monitor.AddButton(button); err != nil {
			log.Fatalf("failed to add button %s to monitor: %v", buttonCfg.Name, err)
		}
	}

	if err := monitor.Start(); err != nil {
		log.Fatalf("failed to start monitor: %v", err)
	}
}
