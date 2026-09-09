package notification

import (
	"sync"

	"nofx/logger"
)

// Handler consumes events. Must not block; long work belongs in a goroutine.
type Handler func(e *Event)

// bus is a minimal synchronous-fanout / async-dispatch event bus.
type bus struct {
	mu       sync.RWMutex
	handlers []Handler
}

var globalBus = &bus{}

// Subscribe registers a handler for all events.
func Subscribe(h Handler) {
	globalBus.mu.Lock()
	defer globalBus.mu.Unlock()
	globalBus.handlers = append(globalBus.handlers, h)
}

// Publish dispatches an event to all subscribers. Each subscriber is invoked
// in its own goroutine so a slow channel never blocks trading; panics in a
// handler are contained.
func Publish(e *Event) {
	if e == nil {
		return
	}
	if e.Time.IsZero() {
		e.Time = timeNowUTC()
	}
	if e.Severity == "" {
		e.Severity = SeverityInfo
	}

	globalBus.mu.RLock()
	handlers := make([]Handler, len(globalBus.handlers))
	copy(handlers, globalBus.handlers)
	globalBus.mu.RUnlock()

	for _, h := range handlers {
		go func(h Handler) {
			defer func() {
				if r := recover(); r != nil {
					logger.Errorf("notification: handler panic: %v", r)
				}
			}()
			h(e)
		}(h)
	}
}
