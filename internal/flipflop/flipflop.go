package flipflop

import (
	"log"
	"sync"
	"time"

	"github.com/larsks/airdancer/internal/switchcollection"
)

// Flipflop represents a rotating output that cycles through switches in a group
type Flipflop struct {
	switches  []switchcollection.Switch
	period    float64
	dutyCycle float64
	stopCh    chan struct{}
	doneCh    chan struct{}
	mutex     sync.RWMutex
	running   bool
	current   int // Index of currently active switch
}

// NewFlipflop creates a new Flipflop instance with the given switches and period in seconds
func NewFlipflop(switches []switchcollection.Switch, period float64, dutyCycle float64) (*Flipflop, error) {
	if len(switches) == 0 {
		return nil, ErrNoSwitches
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

	return &Flipflop{
		switches:  switches,
		period:    period,
		dutyCycle: dutyCycle,
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
		current:   -1, // Start with no switch active
	}, nil
}

// Start begins the flipflop operation
func (f *Flipflop) Start() error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if f.running {
		return ErrAlreadyRunning
	}

	f.running = true
	go f.flipflopLoop()

	return nil
}

// Stop stops the flipflop operation and ensures all switches are turned off
func (f *Flipflop) Stop() error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if !f.running {
		return ErrNotRunning
	}

	close(f.stopCh)
	<-f.doneCh // Wait for goroutine to finish

	// Ensure all switches are off when we stop
	for _, sw := range f.switches {
		if err := sw.TurnOff(); err != nil {
			log.Printf("flipflop failed to turn off switch %s during stop: %v", sw, err)
		}
	}

	f.running = false
	f.current = -1

	// Reset channels for potential restart
	f.stopCh = make(chan struct{})
	f.doneCh = make(chan struct{})

	return nil
}

// IsRunning returns true if the flipflop is currently running
func (f *Flipflop) IsRunning() bool {
	f.mutex.RLock()
	defer f.mutex.RUnlock()
	return f.running
}

// GetPeriod returns the current period in seconds
func (f *Flipflop) GetPeriod() float64 {
	f.mutex.RLock()
	defer f.mutex.RUnlock()
	return f.period
}

// GetDutyCycle returns the current duty cycle
func (f *Flipflop) GetDutyCycle() float64 {
	f.mutex.RLock()
	defer f.mutex.RUnlock()
	return f.dutyCycle
}

// GetSwitches returns the underlying switches
func (f *Flipflop) GetSwitches() []switchcollection.Switch {
	f.mutex.RLock()
	defer f.mutex.RUnlock()
	return f.switches
}

// GetCurrentSwitch returns the index of the currently active switch (-1 if none)
func (f *Flipflop) GetCurrentSwitch() int {
	f.mutex.RLock()
	defer f.mutex.RUnlock()
	return f.current
}

// flipflopLoop is the main goroutine that handles the rotation
func (f *Flipflop) flipflopLoop() {
	defer close(f.doneCh)

	onTime := time.Duration(f.period * f.dutyCycle * float64(time.Second))
	offTime := time.Duration(f.period * (1 - f.dutyCycle) * float64(time.Second))
	clock := time.NewTimer(offTime)
	defer clock.Stop()

	state := false // Start with off state

	for {
		select {
		case <-f.stopCh:
			return
		case <-clock.C:
			state = !state
			if state {
				// Turn on the next switch in sequence
				f.current = (f.current + 1) % len(f.switches)
				sw := f.switches[f.current]
				if err := sw.TurnOn(); err != nil {
					log.Printf("flipflop failed to turn on switch %s", sw)
					break
				}
				clock = time.NewTimer(onTime)
			} else {
				// Turn off the current switch
				if f.current >= 0 && f.current < len(f.switches) {
					sw := f.switches[f.current]
					if err := sw.TurnOff(); err != nil {
						log.Printf("flipflop failed to turn off switch %s", sw)
						break
					}
				}
				clock = time.NewTimer(offTime)
			}
		}
	}
}
