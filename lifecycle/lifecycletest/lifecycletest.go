// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MPL-2.0

// Package lifecycletest provides helpers for testing components that use the
// lifecycle package: a manual clock, a recording observer and assertions.
package lifecycletest

import (
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/consul/lifecycle"
)

// Clock is a manual lifecycle.Clock. Timers fire only from Advance.
type Clock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*timer
}

type timer struct {
	clock   *Clock
	at      time.Time
	f       func()
	stopped bool
}

func (t *timer) Stop() bool {
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()
	was := !t.stopped
	t.stopped = true
	return was
}

// NewClock returns a Clock starting at a fixed time.
func NewClock() *Clock {
	return &Clock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
}

// Now implements lifecycle.Clock.
func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// AfterFunc implements lifecycle.Clock.
func (c *Clock) AfterFunc(d time.Duration, f func()) lifecycle.Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &timer{clock: c, at: c.now.Add(d), f: f}
	c.timers = append(c.timers, t)
	return t
}

// Advance moves time forward and runs due timers, in time order, without
// holding the clock lock.
func (c *Clock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	var due, keep []*timer
	for _, t := range c.timers {
		switch {
		case t.stopped:
		case !t.at.After(c.now):
			t.stopped = true
			due = append(due, t)
		default:
			keep = append(keep, t)
		}
	}
	c.timers = keep
	c.mu.Unlock()
	sort.Slice(due, func(i, j int) bool { return due[i].at.Before(due[j].at) })
	for _, t := range due {
		t.f()
	}
}

// Recorder is an Observer that records every notification.
type Recorder struct {
	lifecycle.NoopObserver

	mu      sync.Mutex
	events  []lifecycle.ControlEvent
	stuck   []lifecycle.Status
	drains  []lifecycle.Status
	invalid []string
}

// OnEvent implements lifecycle.Observer.
func (r *Recorder) OnEvent(ev lifecycle.ControlEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, ev)
}

// OnStuck implements lifecycle.Observer.
func (r *Recorder) OnStuck(st lifecycle.Status, _ time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stuck = append(r.stuck, st)
}

// OnDrainTimeout implements lifecycle.Observer.
func (r *Recorder) OnDrainTimeout(st lifecycle.Status, _ int, _ time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.drains = append(r.drains, st)
}

// OnInvalidTransition implements lifecycle.Observer.
func (r *Recorder) OnInvalidTransition(c lifecycle.ComponentName, from, to lifecycle.StateName, _ error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.invalid = append(r.invalid, string(c)+":"+string(from)+"->"+string(to))
}

// Events returns a copy of the recorded events.
func (r *Recorder) Events() []lifecycle.ControlEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]lifecycle.ControlEvent(nil), r.events...)
}

// Stuck returns the statuses reported stuck.
func (r *Recorder) Stuck() []lifecycle.Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]lifecycle.Status(nil), r.stuck...)
}

// DrainTimeouts returns the statuses reported for drain timeouts.
func (r *Recorder) DrainTimeouts() []lifecycle.Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]lifecycle.Status(nil), r.drains...)
}

// Invalid returns rejected transitions as "component:from->to".
func (r *Recorder) Invalid() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.invalid...)
}

// States returns the states the component entered, in order, starting with
// its initial state at registration.
func (r *Recorder) States(c lifecycle.ComponentName) []lifecycle.StateName {
	var out []lifecycle.StateName
	for _, ev := range r.Events() {
		if ev.Component() != c {
			continue
		}
		switch ev.Kind {
		case lifecycle.EventRegistered, lifecycle.EventTransition:
			out = append(out, ev.To.State)
		}
	}
	return out
}

// Eventually polls cond until it is true or the timeout passes. Observers are
// called asynchronously, so assertions on a Recorder should use it.
func Eventually(t testing.TB, timeout time.Duration, cond func() bool, msgAndArgs ...interface{}) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	if len(msgAndArgs) > 0 {
		t.Fatalf("condition not met within %s: "+msgAndArgs[0].(string), append([]interface{}{timeout}, msgAndArgs[1:]...)...)
	}
	t.Fatalf("condition not met within %s", timeout)
}

// AssertTransitions waits until the recorded states of the component equal
// want.
func AssertTransitions(t testing.TB, r *Recorder, c lifecycle.ComponentName, want ...lifecycle.StateName) {
	t.Helper()
	var got []lifecycle.StateName
	Eventually(t, 2*time.Second, func() bool {
		got = r.States(c)
		if len(got) != len(want) {
			return false
		}
		for i := range got {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	}, "%s states %v, want %v", c, &got, want)
}

// AssertNeverStuck fails if the component was ever reported stuck.
func AssertNeverStuck(t testing.TB, r *Recorder, c lifecycle.ComponentName) {
	t.Helper()
	for _, st := range r.Stuck() {
		if st.Component == c {
			t.Fatalf("%s was reported stuck in %s since %s", c, st.State, st.Since)
		}
	}
}

// AssertDrained fails unless no lease is held on the component.
func AssertDrained(t testing.TB, ex *lifecycle.Exchange, c lifecycle.ComponentName) {
	t.Helper()
	if n := ex.Leases(c); n != 0 {
		t.Fatalf("%s still has %d leases", c, n)
	}
}
