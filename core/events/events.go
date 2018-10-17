package events

import (
	"github.com/TIBCOSoftware/flogo-lib/logger"
	"sync"
	"runtime/debug"
	"errors"
)

type EventListener interface {
	// Returns name of the listener
	Name() string

	// Called when matching event occurs
	OnEvent(*EventContext) error
}

var eventListeners = make(map[string]EventListener)

// Buffered channel
var eventQueue = make(chan *EventContext, 100)
var publisherRoutineStarted = false
var shutdown = make(chan bool)

var lock = &sync.RWMutex{}

// Registers listener for events
func RegisterEventListener(evtListener EventListener) error {
	lock.Lock()
	defer lock.Unlock()
	_, exists := eventListeners[evtListener.Name()]
	if exists {
		return errors.New("Failed to register event listener. Listener already registered with given name.")
	}

	eventListeners[evtListener.Name()] = evtListener
	logger.Debugf("Event Listener - '%s' successfully registered", evtListener.Name())
	startPublisherRoutine()
	return nil
}

// Unregisters event listener
func UnRegisterEventListener(name string) {
	lock.Lock()
	defer lock.Unlock()
	_, exists := eventListeners[name]
	if exists {
		delete(eventListeners, name)
		logger.Debugf("Event Listener - '%s' successfully unregistered", name)
		stopPublisherRoutine()
	}
}

func startPublisherRoutine() {
	if publisherRoutineStarted == true {
		return
	}

	if len(eventListeners) > 0 {
		// start publisher routine
		go publishEvents()
		publisherRoutineStarted = true
	}
}

func stopPublisherRoutine() {
	if publisherRoutineStarted == false {
		return
	}

	if len(eventListeners) == 0 {
		// No more listeners. Stop go routine
		close(shutdown)
		publisherRoutineStarted = false
	}
}

//  EventContext is a wrapper over specific event
type EventContext struct {
	// Type of event
	eventType string
	// Event data
	event interface{}
}

// Returns wrapped event data
func (ec *EventContext) GetEvent() interface{} {
	return ec.event
}

// Returns event type
func (ec *EventContext) GetType() string {
	return ec.eventType
}

func publishEvents() {
	for {
		select {
		case event := <-eventQueue:
			lock.RLock()
			publishEvent(event)
			lock.RUnlock()
		case <-shutdown:
			return
		}
	}
}

func publishEvent(fe *EventContext) {
	for _, ls := range eventListeners {
		func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Errorf("Registered event handler - '%s' failed to process event due to error - '%v' ", ls.Name(), r)
					logger.Errorf("StackTrace: %s", debug.Stack())
				}
			}()
			err := ls.OnEvent(fe)
			if err != nil {
				logger.Errorf("Registered event handler - '%s' failed to process event due to error - '%s' ", ls.Name(), err.Error())
			}
		}()
	}
}

//TODO channel to be passed to actions
// Puts event with given type and data on the channel
func PublishEvent(eType string, event interface{}) {
	evtContext := &EventContext{event: event, eventType: eType}
	// Put event on the queue
	eventQueue <- evtContext
}
