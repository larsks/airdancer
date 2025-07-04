package blink

import (
	"log"
	"sync"
	"time"

	"github.com/larsks/airdancer/internal/switchcollection"
)

// Blink represents a blinking output that toggles a Switch at a given frequency
type Blink struct {
	sw        switchcollection.Switch
	frequency float64
	period    time.Duration
	stopCh    chan struct{}
	doneCh    chan struct{}
	mutex     sync.RWMutex
	running   bool
}

// NewBlink creates a new Blink instance with the given switch and frequency in hertz
func NewBlink(sw switchcollection.Switch, frequency float64) (*Blink, error) {
	if sw == nil {
		return nil, ErrSwitchRequired
	}
	if frequency <= 0 {
		return nil, ErrInvalidFrequency
	}

	// Calculate period from frequency (frequency = 1/period)
	period := time.Duration(float64(time.Second) / frequency)

	return &Blink{
		sw:        sw,
		frequency: frequency,
		period:    period,
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}, nil
}

// Start begins the blinking operation
func (b *Blink) Start() error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.running {
		return ErrAlreadyRunning
	}

	b.running = true
	go b.blinkLoop()

	return nil
}

// Stop stops the blinking operation and ensures the switch is turned off
func (b *Blink) Stop() error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if !b.running {
		return ErrNotRunning
	}

	close(b.stopCh)
	<-b.doneCh // Wait for goroutine to finish

	// Ensure switch is off when we stop
	err := b.sw.TurnOff()
	if err != nil {
		return err
	}

	b.running = false

	// Reset channels for potential restart
	b.stopCh = make(chan struct{})
	b.doneCh = make(chan struct{})

	return nil
}

// IsRunning returns true if the blink is currently running
func (b *Blink) IsRunning() bool {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.running
}

// GetFrequency returns the current frequency in hertz
func (b *Blink) GetFrequency() float64 {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.frequency
}

// GetSwitch returns the underlying switch
func (b *Blink) GetSwitch() switchcollection.Switch {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.sw
}

// blinkLoop is the main goroutine that handles the blinking
func (b *Blink) blinkLoop() {
	defer close(b.doneCh)

	ticker := time.NewTicker(b.period / 2) // Half period for on/off cycle
	defer ticker.Stop()

	state := false // Start with off state

	for {
		select {
		case <-b.stopCh:
			return
		case <-ticker.C:
			state = !state
			if state {
				if err := b.sw.TurnOn(); err != nil {
					log.Printf("blinker failed to turn on switch %s", b.sw)
					break
				}
			} else {
				if err := b.sw.TurnOff(); err != nil {
					log.Printf("blinker failed to turn off switch %s", b.sw)
					break
				}
			}
		}
	}
}
