package consul

import (
	"sync"
	"time"
)

// SessionTimers provides a map of named timers which
// is safe for concurrent use.
type SessionTimers struct {
	sync.RWMutex
	m map[string]*time.Timer
}

func NewSessionTimers() *SessionTimers {
	return &SessionTimers{m: make(map[string]*time.Timer)}
}

// Get returns the timer with the given id or nil.
func (t *SessionTimers) Get(id string) *time.Timer {
	t.RLock()
	defer t.RUnlock()
	return t.m[id]
}

// Set stores the timer under given id. If tm is nil the timer
// with the given id is removed.
func (t *SessionTimers) Set(id string, tm *time.Timer) {
	t.Lock()
	defer t.Unlock()
	if tm == nil {
		delete(t.m, id)
	} else {
		t.m[id] = tm
	}
}

// Del removes the timer with the given id.
func (t *SessionTimers) Del(id string) {
	t.Set(id, nil)
}

// Len returns the number of registered timers.
func (t *SessionTimers) Len() int {
	t.RLock()
	defer t.RUnlock()
	return len(t.m)
}

// ResetOrCreate sets the ttl of the timer with the given id or creates a new
// one if it does not exist.
func (t *SessionTimers) ResetOrCreate(id string, ttl time.Duration, afterFunc func()) {
	t.Lock()
	defer t.Unlock()

	if tm := t.m[id]; tm != nil {
		tm.Reset(ttl)
		return
	}
	t.m[id] = time.AfterFunc(ttl, afterFunc)
}

// Stop stops the timer with the given id and removes it.
func (t *SessionTimers) Stop(id string) {
	t.Lock()
	defer t.Unlock()
	if tm := t.m[id]; tm != nil {
		tm.Stop()
		delete(t.m, id)
	}
}

// StopAll stops and removes all registered timers.
func (t *SessionTimers) StopAll() {
	t.Lock()
	defer t.Unlock()
	for _, tm := range t.m {
		tm.Stop()
	}
	t.m = make(map[string]*time.Timer)
}
