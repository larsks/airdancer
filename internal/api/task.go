package api

import (
	"fmt"
	"log"
	"sync"

	"github.com/larsks/airdancer/internal/blink"
	"github.com/larsks/airdancer/internal/flipflop"
)

// Task represents a long-running switch operation
type Task interface {
	Start() error
	Stop() error
	IsRunning() bool
	Type() TaskType
}

type TaskType string

const (
	TaskTypeBlink    TaskType = "blink"
	TaskTypeFlipflop TaskType = "flipflop"
)

// TaskManager handles task lifecycle and events
type TaskManager struct {
	server *Server
	tasks  map[string]Task // key is switch/group name
	mutex  sync.RWMutex
}

// NewTaskManager creates a new TaskManager
func NewTaskManager(server *Server) *TaskManager {
	return &TaskManager{
		server: server,
		tasks:  make(map[string]Task),
	}
}

// StartTask starts a task and emits the appropriate MQTT event
func (tm *TaskManager) StartTask(name string, task Task) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	// Stop any existing task for this switch/group
	if err := tm.stopTaskUnsafe(name); err != nil {
		return err
	}

	// Start the new task
	if err := task.Start(); err != nil {
		return err
	}

	tm.tasks[name] = task
	log.Printf("start %s on %s", task.Type(), name)

	// Emit start event
	eventName := string(task.Type())
	tm.server.publishMQTTSwitchEvent(name, eventName)

	return nil
}

// StopTask stops a task and emits an "off" MQTT event
func (tm *TaskManager) StopTask(name string) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()
	return tm.stopTaskUnsafe(name)
}

// stopTaskUnsafe stops a task without acquiring the mutex (for internal use)
func (tm *TaskManager) stopTaskUnsafe(name string) error {
	task, exists := tm.tasks[name]
	if !exists {
		return nil
	}

	if task.IsRunning() {
		log.Printf("canceling %s on %s", task.Type(), name)
		if err := task.Stop(); err != nil {
			return fmt.Errorf("failed to cancel %s on %s: %w", task.Type(), name, err)
		}
		// Emit stop event
		tm.server.publishSwitchStateChange(name, false)
	}

	delete(tm.tasks, name)
	return nil
}

// GetTask returns the task for a given name (if any)
func (tm *TaskManager) GetTask(name string) (Task, bool) {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()
	task, exists := tm.tasks[name]
	return task, exists
}

// StopAllTasks stops all running tasks
func (tm *TaskManager) StopAllTasks() error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	errorCollector := NewErrorCollector()
	for name := range tm.tasks {
		if err := tm.stopTaskUnsafe(name); err != nil {
			errorCollector.Add(fmt.Sprintf("task %s", name), err)
		}
	}

	return errorCollector.Result("errors stopping tasks")
}

// BlinkTask wraps a blink.Blink as a Task
type BlinkTask struct {
	blink *blink.Blink
}

// NewBlinkTask creates a new BlinkTask
func NewBlinkTask(blink *blink.Blink) *BlinkTask {
	return &BlinkTask{blink: blink}
}

func (bt *BlinkTask) Start() error          { return bt.blink.Start() }
func (bt *BlinkTask) Stop() error           { return bt.blink.Stop() }
func (bt *BlinkTask) IsRunning() bool       { return bt.blink.IsRunning() }
func (bt *BlinkTask) Type() TaskType        { return TaskTypeBlink }
func (bt *BlinkTask) GetPeriod() float64    { return bt.blink.GetPeriod() }
func (bt *BlinkTask) GetDutyCycle() float64 { return bt.blink.GetDutyCycle() }

// FlipflopTask wraps a flipflop.Flipflop as a Task
type FlipflopTask struct {
	flipflop *flipflop.Flipflop
}

// NewFlipflopTask creates a new FlipflopTask
func NewFlipflopTask(flipflop *flipflop.Flipflop) *FlipflopTask {
	return &FlipflopTask{flipflop: flipflop}
}

func (ft *FlipflopTask) Start() error          { return ft.flipflop.Start() }
func (ft *FlipflopTask) Stop() error           { return ft.flipflop.Stop() }
func (ft *FlipflopTask) IsRunning() bool       { return ft.flipflop.IsRunning() }
func (ft *FlipflopTask) Type() TaskType        { return TaskTypeFlipflop }
func (ft *FlipflopTask) GetPeriod() float64    { return ft.flipflop.GetPeriod() }
func (ft *FlipflopTask) GetDutyCycle() float64 { return ft.flipflop.GetDutyCycle() }
