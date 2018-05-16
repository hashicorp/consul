// Package up is used to run a function for some duration. If a new function is added while a previous run is
// still ongoing, nothing new will be executed.
package up

import (
	"sync"
	"time"
)

// Probe is used to run a single Func until it returns true (indicating a target is healthy). If an Func
// is already in progress no new one will be added, i.e. there is always a maximum of 1 checks in flight.
type Probe struct {
	sync.Mutex
	inprogress int
	interval   time.Duration
}

// Func is used to determine if a target is alive. If so this function must return nil.
type Func func() error

// New returns a pointer to an intialized Probe.
func New() *Probe { return &Probe{} }

// Do will probe target, if a probe is already in progress this is a noop.
func (p *Probe) Do(f Func) {
	p.Lock()
	if p.inprogress != idle {
		p.Unlock()
		return
	}
	p.inprogress = active
	interval := p.interval
	p.Unlock()
	// Passed the lock. Now run f for as long it returns false. If a true is returned
	// we return from the goroutine and we can accept another Func to run.
	go func() {
		for {
			if err := f(); err == nil {
				break
			}
			time.Sleep(interval)
			p.Lock()
			if p.inprogress == stop {
				p.Unlock()
				return
			}
			p.Unlock()
		}

		p.Lock()
		p.inprogress = idle
		p.Unlock()
	}()
}

// Stop stops the probing.
func (p *Probe) Stop() {
	p.Lock()
	p.inprogress = stop
	p.Unlock()
}

// Start will initialize the probe manager, after which probes can be initiated with Do.
func (p *Probe) Start(interval time.Duration) { p.SetInterval(interval) }

// SetInterval sets the probing interval to be used by upcoming probes initiated with Do.
func (p *Probe) SetInterval(interval time.Duration) {
	p.Lock()
	p.interval = interval
	p.Unlock()
}

const (
	idle = iota
	active
	stop
)
