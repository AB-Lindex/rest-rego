package filecache

// some fsnotify-events trigger multiple times for each change
// (create usually does a write, but not always)
//
// this package help to de-dup the bubbling of events

import (
	"sync"
	"time"
)

type delayedCallbacks struct {
	fn      func(string)
	delay   time.Duration
	pending map[string]*time.Timer
	mutex   sync.Mutex
}

func newDelayedCallbacks(delay time.Duration, fn func(string)) *delayedCallbacks {
	q := &delayedCallbacks{
		fn:      fn,
		delay:   delay,
		pending: make(map[string]*time.Timer),
	}
	return q
}

func (q *delayedCallbacks) add(name string) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	// If there's already a timer for this name, stop it
	if timer, exists := q.pending[name]; exists {
		timer.Stop()
	}

	// Create a new timer for this name
	q.pending[name] = time.AfterFunc(q.delay, func() {
		q.mutex.Lock()
		delete(q.pending, name)
		q.mutex.Unlock()
		q.fn(name)
	})
}
