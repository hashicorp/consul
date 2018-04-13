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
	do   chan Func
	stop chan bool

	target string

	sync.Mutex
	inprogress int
}

// Func is used to determine if a target is alive. If so this function must return nil.
type Func func() error

// New returns a pointer to an intialized Probe.
func New() *Probe {
	return &Probe{stop: make(chan bool), do: make(chan Func)}
}

// Do will probe target, if a probe is already in progress this is a noop.
func (p *Probe) Do(f Func) { p.do <- f }

// Stop stops the probing.
func (p *Probe) Stop() { p.stop <- true }

// Start will start the probe manager, after which probes can be initialized with Do.
func (p *Probe) Start(interval time.Duration) { go p.start(interval) }

func (p *Probe) start(interval time.Duration) {
	for {
		select {
		case <-p.stop:
			p.Lock()
			p.inprogress = stop
			p.Unlock()
			return
		case f := <-p.do:
			p.Lock()
			if p.inprogress == active || p.inprogress == stop {
				p.Unlock()
				continue
			}
			p.inprogress = active
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
	}
}

const (
	idle = iota
	active
	stop
)
