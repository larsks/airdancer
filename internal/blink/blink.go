package blink

import (
	"log"
	"sync"
	"time"

	"github.com/larsks/airdancer/internal/switchcollection"
)

// Blink represents a blinking output that toggles a Switch at a given period
type Blink struct {
	sw        switchcollection.Switch
	period    float64
	dutyCycle float64
	stopCh    chan struct{}
	doneCh    chan struct{}
	mutex     sync.RWMutex
	running   bool
}

// NewBlink creates a new Blink instance with the given switch and period in seconds
func NewBlink(sw switchcollection.Switch, period float64, dutyCycle float64) (*Blink, error) {
	if sw == nil {
		return nil, ErrSwitchRequired
	}
	if period <= 0 {
		return nil, ErrInvalidPeriod
	}
	if dutyCycle < 0 || dutyCycle > 1 {
		return nil, ErrInvalidDutyCycle
	}

	// Default duty cycle is 0.5 if not specified
	if dutyCycle == 0 {
		dutyCycle = 0.5
	}

	return &Blink{
		sw:        sw,
		period:    period,
		dutyCycle: dutyCycle,
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

// GetPeriod returns the current period in seconds
func (b *Blink) GetPeriod() float64 {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.period
}

// GetDutyCycle returns the current duty cycle
func (b *Blink) GetDutyCycle() float64 {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.dutyCycle
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

	onTime := time.Duration(b.period * b.dutyCycle * float64(time.Second))
	offTime := time.Duration(b.period * (1 - b.dutyCycle) * float64(time.Second))
	clock := time.NewTimer(offTime)
	defer clock.Stop()

	state := false // Start with off state

	for {
		select {
		case <-b.stopCh:
			return
		case <-clock.C:
			state = !state
			if state {
				if err := b.sw.TurnOn(); err != nil {
					log.Printf("blinker failed to turn on switch %s", b.sw)
					break
				}
				clock = time.NewTimer(onTime)
			} else {
				if err := b.sw.TurnOff(); err != nil {
					log.Printf("blinker failed to turn off switch %s", b.sw)
					break
				}
				clock = time.NewTimer(offTime)
			}
		}
	}
}
