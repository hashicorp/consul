// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MPL-2.0

package lifecycle

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Observer receives lifecycle notifications. Observers are called from one
// goroutine, in order, outside all Exchange locks; a panic is recovered and
// logged. Embed NoopObserver so new methods added later do not break you.
type Observer interface {
	// OnEvent is called for every ControlEvent.
	OnEvent(ev ControlEvent)
	// OnStuck is called once when a component stays in a state longer than the
	// state's MaxDuration.
	OnStuck(st Status, after time.Duration)
	// OnDrainTimeout is called when BeginTransition stops waiting while leases
	// are still held.
	OnDrainTimeout(st Status, held int, waited time.Duration)
	// OnInvalidTransition is called when a transition is rejected.
	OnInvalidTransition(component ComponentName, from, to StateName, err error)
}

// NoopObserver implements Observer with no-ops. Embed it to implement only
// the methods you need.
type NoopObserver struct{}

func (NoopObserver) OnEvent(ControlEvent)                                           {}
func (NoopObserver) OnStuck(Status, time.Duration)                                  {}
func (NoopObserver) OnDrainTimeout(Status, int, time.Duration)                      {}
func (NoopObserver) OnInvalidTransition(ComponentName, StateName, StateName, error) {}

type noticeKind int

const (
	noticeEvent noticeKind = iota + 1
	noticeStuck
	noticeDrainTimeout
	noticeInvalid
)

type notice struct {
	kind      noticeKind
	event     ControlEvent
	status    Status
	duration  time.Duration
	held      int
	component ComponentName
	from, to  StateName
	err       error
}

// dispatcher delivers notices to observers from a single goroutine through a
// bounded queue. When the queue is full notices are dropped and counted, so a
// slow observer can never block a component or grow memory.
type dispatcher struct {
	observers []Observer
	logger    Logger
	metrics   Metrics

	mu      sync.Mutex
	ch      chan notice
	closed  bool
	dropped atomic.Uint64
	done    chan struct{}
}

func newDispatcher(observers []Observer, size int, logger Logger, metrics Metrics) *dispatcher {
	d := &dispatcher{
		observers: observers,
		logger:    logger,
		metrics:   metrics,
		ch:        make(chan notice, size),
		done:      make(chan struct{}),
	}
	go d.run()
	return d
}

// enqueue never blocks and never calls out, so it is safe under any lock.
func (d *dispatcher) enqueue(n notice) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return
	}
	select {
	case d.ch <- n:
	default:
		d.dropped.Add(1)
	}
}

func (d *dispatcher) close() {
	d.mu.Lock()
	if !d.closed {
		d.closed = true
		close(d.ch)
	}
	d.mu.Unlock()
	<-d.done
}

func (d *dispatcher) run() {
	defer close(d.done)
	for n := range d.ch {
		for _, o := range d.observers {
			d.deliver(o, n)
		}
		if dropped := d.dropped.Swap(0); dropped > 0 {
			d.metrics.IncrCounter([]string{"lifecycle", "observer_dropped"}, float32(dropped), nil)
			d.logger.Warn("lifecycle observer queue full, notifications dropped", "dropped", dropped)
		}
	}
}

func (d *dispatcher) deliver(o Observer, n notice) {
	defer func() {
		if r := recover(); r != nil {
			d.logger.Error("lifecycle observer panicked", "observer", fmt.Sprintf("%T", o), "panic", r)
		}
	}()
	switch n.kind {
	case noticeEvent:
		o.OnEvent(n.event)
	case noticeStuck:
		o.OnStuck(n.status, n.duration)
	case noticeDrainTimeout:
		o.OnDrainTimeout(n.status, n.held, n.duration)
	case noticeInvalid:
		o.OnInvalidTransition(n.component, n.from, n.to, n.err)
	}
}

type loggingObserver struct{ l Logger }

func (o loggingObserver) OnEvent(ev ControlEvent) {
	args := []interface{}{
		"component", ev.Component(),
		"event", ev.Kind.String(),
		"from", ev.From.State,
		"to", ev.To.State,
		"phase", ev.To.Phase.String(),
		"generation", ev.To.Generation,
		"seq", ev.Seq,
	}
	if ev.To.Reason != "" {
		args = append(args, "reason", ev.To.Reason)
	}
	if ev.Kind == EventTransition && (ev.To.Phase == PhaseTransitioning || ev.From.Phase == PhaseTransitioning ||
		ev.To.Phase == PhaseFailed) {
		o.l.Info("lifecycle transition", args...)
		return
	}
	o.l.Debug("lifecycle event", args...)
}

func (o loggingObserver) OnStuck(st Status, after time.Duration) {
	o.l.Warn("lifecycle component stuck in state", "component", st.Component, "state", st.State,
		"phase", st.Phase.String(), "since", st.Since, "after", after, "reason", st.Reason)
}

func (o loggingObserver) OnDrainTimeout(st Status, held int, waited time.Duration) {
	o.l.Warn("lifecycle transition proceeding with leases still held", "component", st.Component,
		"state", st.State, "leases", held, "waited", waited)
}

func (o loggingObserver) OnInvalidTransition(component ComponentName, from, to StateName, err error) {
	o.l.Warn("lifecycle transition rejected", "component", component, "from", from, "to", to, "error", err)
}

type metricsObserver struct{ m Metrics }

func (o metricsObserver) OnEvent(ev ControlEvent) {
	name := string(ev.Component())
	switch ev.Kind {
	case EventRegistered:
		o.m.SetGauge([]string{"lifecycle", "phase"}, float32(ev.To.Phase), map[string]string{"component": name})
		o.m.SetGauge([]string{"lifecycle", "state"}, 1, map[string]string{"component": name, "state": string(ev.To.State)})
	case EventTransition:
		o.m.SetGauge([]string{"lifecycle", "phase"}, float32(ev.To.Phase), map[string]string{"component": name})
		o.m.SetGauge([]string{"lifecycle", "state"}, 0, map[string]string{"component": name, "state": string(ev.From.State)})
		o.m.SetGauge([]string{"lifecycle", "state"}, 1, map[string]string{"component": name, "state": string(ev.To.State)})
		o.m.IncrCounter([]string{"lifecycle", "transition"}, 1,
			map[string]string{"component": name, "from": string(ev.From.State), "to": string(ev.To.State)})
	case EventRemoved:
		o.m.SetGauge([]string{"lifecycle", "phase"}, float32(PhaseUnknown), map[string]string{"component": name})
		o.m.SetGauge([]string{"lifecycle", "state"}, 0, map[string]string{"component": name, "state": string(ev.From.State)})
	}
}

func (o metricsObserver) OnStuck(st Status, _ time.Duration) {
	o.m.IncrCounter([]string{"lifecycle", "stuck"}, 1, map[string]string{"component": string(st.Component), "state": string(st.State)})
}

func (o metricsObserver) OnDrainTimeout(st Status, _ int, _ time.Duration) {
	o.m.IncrCounter([]string{"lifecycle", "drain_timeout"}, 1, map[string]string{"component": string(st.Component)})
}

func (o metricsObserver) OnInvalidTransition(component ComponentName, _, _ StateName, _ error) {
	o.m.IncrCounter([]string{"lifecycle", "invalid_transition"}, 1, map[string]string{"component": string(component)})
}
