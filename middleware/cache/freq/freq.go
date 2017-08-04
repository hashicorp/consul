// Package freq keeps track of last X seen events. The events themselves are not stored
// here. So the Freq type should be added next to the thing it is tracking.
package freq

import (
	"sync"
	"time"
)

// Freq tracks the frequencies of things.
type Freq struct {
	// Last time we saw a query for this element.
	last time.Time
	// Number of this in the last time slice.
	hits int

	sync.RWMutex
}

// New returns a new initialized Freq.
func New(t time.Time) *Freq {
	return &Freq{last: t, hits: 0}
}

// Update updates the number of hits. Last time seen will be set to now.
// If the last time we've seen this entity is within now - d, we increment hits, otherwise
// we reset hits to 1. It returns the number of hits.
func (f *Freq) Update(d time.Duration, now time.Time) int {
	earliest := now.Add(-1 * d)
	f.Lock()
	defer f.Unlock()
	if f.last.Before(earliest) {
		f.last = now
		f.hits = 1
		return f.hits
	}
	f.last = now
	f.hits++
	return f.hits
}

// Hits returns the number of hits that we have seen, according to the updates we have done to f.
func (f *Freq) Hits() int {
	f.RLock()
	defer f.RUnlock()
	return f.hits
}

// Reset resets f to time t and hits to hits.
func (f *Freq) Reset(t time.Time, hits int) {
	f.Lock()
	defer f.Unlock()
	f.last = t
	f.hits = hits
}
