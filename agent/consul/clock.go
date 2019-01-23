package consul

import (
	"sync"
	"time"
)

// A clock tells the current time.
type clock interface {
	Now() time.Time
}

type stdlibClock int

var standardClock clock = stdlibClock(0)

func (stdlibClock) Now() time.Time {
	return time.Now()
}

type stoppedClock struct {
	mu  sync.RWMutex
	now time.Time
}

func newStoppedClock(now time.Time) *stoppedClock {
	return &stoppedClock{
		now: now,
	}
}

func (c *stoppedClock) Now() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.now
}

// Add changes the value that will be emitted for Now() and returns the prior setting.
func (c *stoppedClock) Add(d time.Duration) time.Time {
	c.mu.Lock()
	old := c.now
	c.now = c.now.Add(d)
	c.mu.Unlock()
	return old
}

func (c *stoppedClock) Set(t time.Time) {
	c.mu.Lock()
	c.now = t
	c.mu.Unlock()
}

func (c *stoppedClock) Reset() {
	c.Set(time.Now())
}
