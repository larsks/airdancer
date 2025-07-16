package event

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/larsks/airdancer/internal/buttondriver/common"
	"github.com/larsks/airdancer/internal/events"
)

// EventButtonDriver implements the common ButtonDriver interface for input event devices
type EventButtonDriver struct {
	buttons   map[string][]*EventButtonSpec
	files     map[string]*os.File
	eventChan chan common.ButtonEvent
	stopChan  chan struct{}
	wg        sync.WaitGroup
	mutex     sync.RWMutex
	started   bool
}

// NewEventButtonDriver creates a new event-based button driver
func NewEventButtonDriver() *EventButtonDriver {
	return &EventButtonDriver{
		buttons:   make(map[string][]*EventButtonSpec),
		files:     make(map[string]*os.File),
		eventChan: make(chan common.ButtonEvent, 100),
		stopChan:  make(chan struct{}),
	}
}

// Events returns the channel for button events
func (d *EventButtonDriver) Events() <-chan common.ButtonEvent {
	return d.eventChan
}

// Start begins monitoring for button events
func (d *EventButtonDriver) Start() error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.started {
		return fmt.Errorf("driver already started")
	}

	if len(d.buttons) == 0 {
		return fmt.Errorf("no buttons configured")
	}

	// Open all device files
	for device := range d.buttons {
		if _, exists := d.files[device]; !exists {
			file, err := os.Open(device)
			if err != nil {
				return fmt.Errorf("failed to open device %s: %v", device, err)
			}
			d.files[device] = file
		}
	}

	d.started = true

	// Start monitoring each device
	for device, buttonSpecs := range d.buttons {
		d.wg.Add(1)
		go d.monitorDevice(device, buttonSpecs)
	}

	return nil
}

// Stop stops monitoring and closes the events channel
func (d *EventButtonDriver) Stop() {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if !d.started {
		return
	}

	d.started = false

	// Close all device files first to unblock any Read operations
	for device, file := range d.files {
		if err := file.Close(); err != nil {
			log.Printf("Error closing device %s: %v", device, err)
		}
	}

	// Signal all goroutines to stop
	close(d.stopChan)

	// Wait for all goroutines to finish
	d.wg.Wait()

	// Close the event channel
	close(d.eventChan)
}

// AddButton adds a button to be monitored
func (d *EventButtonDriver) AddButton(buttonSpec interface{}) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	spec, ok := buttonSpec.(*EventButtonSpec)
	if !ok {
		return fmt.Errorf("invalid button spec type, expected *EventButtonSpec")
	}

	if err := spec.Validate(); err != nil {
		return fmt.Errorf("invalid button specification: %v", err)
	}

	d.buttons[spec.Device] = append(d.buttons[spec.Device], spec)
	return nil
}

// GetButtons returns a list of button sources being monitored
func (d *EventButtonDriver) GetButtons() []string {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	var buttonNames []string
	for _, buttonSpecs := range d.buttons {
		for _, spec := range buttonSpecs {
			buttonNames = append(buttonNames, spec.Name)
		}
	}
	return buttonNames
}

// monitorDevice monitors a single input device for events
func (d *EventButtonDriver) monitorDevice(device string, buttonSpecs []*EventButtonSpec) {
	defer d.wg.Done()

	file := d.files[device]
	log.Printf("Starting monitoring for device: %s with %d button(s)", device, len(buttonSpecs))

	eventSize := int(unsafe.Sizeof(events.InputEvent{}))

	// Create a goroutine to read from the file
	readChan := make(chan []byte, 1)
	errorChan := make(chan error, 1)
	readerDone := make(chan struct{})

	go func() {
		defer close(readerDone)
		for {
			readBuffer := make([]byte, eventSize)
			n, err := file.Read(readBuffer)
			if err != nil {
				select {
				case errorChan <- err:
				case <-d.stopChan:
				}
				return
			}
			if n == eventSize {
				select {
				case readChan <- readBuffer:
				case <-d.stopChan:
					return
				}
			}
		}
	}()

	for {
		select {
		case <-d.stopChan:
			log.Printf("Stopping monitoring for device: %s", device)
			return
		case err := <-errorChan:
			// Check if this is an expected error due to shutdown
			if strings.Contains(err.Error(), "file already closed") {
				log.Printf("Stopping monitoring for device: %s", device)
			} else {
				log.Printf("Error reading from device %s: %v", device, err)
			}
			return
		case buffer := <-readChan:
			// Parse the input event
			var inputEvent events.InputEvent
			err := binary.Read(bytes.NewReader(buffer), binary.LittleEndian, &inputEvent)
			if err != nil {
				log.Printf("Error parsing event from %s: %v", device, err)
				continue
			}

			// Process the event for each matching button
			for _, spec := range buttonSpecs {
				if events.EventType(inputEvent.Type) == spec.EventType && uint32(inputEvent.Code) == spec.EventCode {
					d.handleButtonEvent(spec, uint32(inputEvent.Value), device)
				}
			}
		}
	}
}

// handleButtonEvent processes a button event and sends it to the event channel
func (d *EventButtonDriver) handleButtonEvent(spec *EventButtonSpec, value uint32, device string) {
	var eventType common.ButtonEventType
	var validEvent bool

	switch value {
	case spec.HighValue:
		eventType = common.ButtonPressed
		validEvent = true
	case spec.LowValue:
		eventType = common.ButtonReleased
		validEvent = true
	default:
		// Ignore other values
		return
	}

	if !validEvent {
		return
	}

	event := common.ButtonEvent{
		Source:    spec.Name,
		Type:      eventType,
		Timestamp: time.Now(),
		Device:    device,
		Metadata: map[string]interface{}{
			"event_type": spec.EventType,
			"event_code": spec.EventCode,
			"value":      value,
		},
	}

	select {
	case d.eventChan <- event:
	case <-d.stopChan:
		return
	default:
		log.Printf("Warning: event channel full, dropping event for button %s", spec.Name)
	}
}
