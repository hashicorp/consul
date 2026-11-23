// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MPL-2.0

package lifecycle

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Exchange is the per-process registry of component lifecycles.
//
// A nil *Exchange is valid: every method is a no-op, Register returns a nil
// *Handle (also a no-op) and gates always succeed. This lets code take an
// optional Exchange without nil checks.
//
// Lock order: Exchange.mu, then component.mu, then Subscription.mu. Nothing is
// called outside the package (observers, predicates) while holding any of them.
type Exchange struct {
	cfg config

	mu     sync.RWMutex
	comps  map[ComponentName]*component
	seq    uint64
	subs   map[*Subscription]struct{}
	regCh  chan struct{} // closed and replaced on every Register
	closed bool

	disp *dispatcher
}

type component struct {
	spec ComponentSpec

	mu       sync.Mutex
	status   Status
	removed  bool
	leases   int
	draining bool
	drained  chan struct{} // closed when leases reach zero during a drain
	changed  chan struct{} // closed and replaced on every change
	version  uint64        // bumped on every change (state, attributes, drain, removal)
	timer    Timer
	token    uint64 // bumped on every state change, guards the stuck timer
}

// Handle is returned by Register and is used by the component to report its
// lifecycle. A nil *Handle is a no-op.
type Handle struct {
	ex *Exchange
	c  *component
}

// Done finishes a transition started with BeginTransition.
type Done func(final StateName, opts ...TransitionOption) error

// NewExchange creates an Exchange. Call Close when the process shuts down to
// stop the observer goroutine and timers.
func NewExchange(opts ...Option) *Exchange {
	cfg := defaultConfig()
	for _, o := range opts {
		o(&cfg)
	}
	e := &Exchange{
		cfg:   cfg,
		comps: make(map[ComponentName]*component),
		subs:  make(map[*Subscription]struct{}),
		regCh: make(chan struct{}),
	}

	var observers []Observer
	if cfg.hasLogger {
		observers = append(observers, loggingObserver{l: cfg.logger})
	}
	if cfg.hasMetrics {
		observers = append(observers, metricsObserver{m: cfg.metrics})
	}
	observers = append(observers, cfg.observers...)
	if len(observers) > 0 {
		e.disp = newDispatcher(observers, cfg.observerQueue, cfg.logger, cfg.metrics)
	}
	return e
}

// Close stops the Exchange: subscriptions end, timers stop and the observer
// goroutine exits after delivering what is queued. Leases keep working.
func (e *Exchange) Close() {
	if e == nil {
		return
	}
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return
	}
	e.closed = true
	for _, c := range e.comps {
		c.mu.Lock()
		if c.timer != nil {
			c.timer.Stop()
			c.timer = nil
		}
		c.mu.Unlock()
	}
	subs := e.subs
	e.subs = make(map[*Subscription]struct{})
	for s := range subs {
		s.markClosed()
	}
	e.mu.Unlock()

	if e.disp != nil {
		e.disp.close()
	}
}

// Register adds a component. The spec is validated and copied.
func (e *Exchange) Register(spec ComponentSpec, opts ...TransitionOption) (*Handle, error) {
	if e == nil {
		return nil, nil
	}
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	tc := transitionConfig{}
	for _, o := range opts {
		o(&tc)
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return nil, ErrClosed
	}
	if _, ok := e.comps[spec.Name]; ok {
		return nil, fmt.Errorf("%w: %s", ErrAlreadyRegistered, spec.Name)
	}
	if len(e.comps) >= e.cfg.maxComponents {
		return nil, fmt.Errorf("%w: limit %d", ErrTooManyComponents, e.cfg.maxComponents)
	}
	attrs, err := e.applyAttributes(nil, tc)
	if err != nil {
		return nil, err
	}

	spec = spec.clone()
	st := spec.States[spec.Initial]
	c := &component{
		spec:    spec,
		changed: make(chan struct{}),
		status: Status{
			Component:  spec.Name,
			State:      spec.Initial,
			Phase:      st.Phase,
			Since:      e.cfg.clock.Now(),
			Reason:     tc.reason,
			Attributes: attrs,
		},
	}
	if st.Phase == PhaseReady {
		c.status.Generation = 1
	}
	c.armTimerLocked(e)
	e.comps[spec.Name] = c
	close(e.regCh)
	e.regCh = make(chan struct{})

	e.emitLocked(ControlEvent{Kind: EventRegistered, To: c.status.clone()})
	return &Handle{ex: e, c: c}, nil
}

// Status returns the current status of a component.
func (e *Exchange) Status(name ComponentName) (Status, bool) {
	if e == nil {
		return Status{}, false
	}
	c := e.lookup(name)
	if c == nil {
		return Status{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.removed {
		return Status{}, false
	}
	return c.status.clone(), true
}

// Statuses returns the status of every component, sorted by name.
func (e *Exchange) Statuses() []Status {
	if e == nil {
		return nil
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.statusesLocked(subscribeConfig{})
}

func (e *Exchange) statusesLocked(filter subscribeConfig) []Status {
	out := make([]Status, 0, len(e.comps))
	for name, c := range e.comps {
		if !filter.matches(name) {
			continue
		}
		c.mu.Lock()
		out = append(out, c.status.clone())
		c.mu.Unlock()
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Component < out[j].Component })
	return out
}

// Leases returns the number of leases currently held on a component.
func (e *Exchange) Leases(name ComponentName) int {
	if e == nil {
		return 0
	}
	c := e.lookup(name)
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.leases
}

func (e *Exchange) lookup(name ComponentName) *component {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.comps[name]
}

// emitLocked assigns the next sequence number and fans the event out to
// subscriptions and observers. e.mu must be held for writing.
func (e *Exchange) emitLocked(ev ControlEvent) {
	e.seq++
	ev.Seq = e.seq
	ev.SchemaVersion = SchemaVersion
	for s := range e.subs {
		s.push(ev)
	}
	if e.disp != nil {
		e.disp.enqueue(notice{kind: noticeEvent, event: ev})
	}
}

func (e *Exchange) applyAttributes(current map[string]string, tc transitionConfig) (map[string]string, error) {
	if tc.attributes == nil && !tc.replace {
		return current, nil
	}
	out := make(map[string]string, len(current)+len(tc.attributes))
	if !tc.replace {
		for k, v := range current {
			out[k] = v
		}
	}
	for k, v := range tc.attributes {
		if v == "" {
			delete(out, k)
			continue
		}
		if len(k) > e.cfg.maxAttrKey || len(v) > e.cfg.maxAttrValue {
			return nil, fmt.Errorf("%w: key %q exceeds %d/%d bytes", ErrAttributesTooLarge, k, e.cfg.maxAttrKey, e.cfg.maxAttrValue)
		}
		out[k] = v
	}
	if len(out) > e.cfg.maxAttrs {
		return nil, fmt.Errorf("%w: %d keys, limit %d", ErrAttributesTooLarge, len(out), e.cfg.maxAttrs)
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// armTimerLocked (re)starts the stuck watchdog for the current state.
// c.mu or exclusive access is required.
func (c *component) armTimerLocked(e *Exchange) {
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
	c.token++
	max := c.spec.States[c.status.State].MaxDuration
	if max <= 0 || e.closed {
		return
	}
	token := c.token
	c.timer = e.cfg.clock.AfterFunc(max, func() { e.fireStuck(c, token, max) })
}

func (e *Exchange) fireStuck(c *component, token uint64, after time.Duration) {
	c.mu.Lock()
	if c.removed || c.token != token {
		c.mu.Unlock()
		return
	}
	st := c.status.clone()
	c.mu.Unlock()
	if e.disp != nil {
		e.disp.enqueue(notice{kind: noticeStuck, status: st, duration: after})
	}
}

// Name returns the component name.
func (h *Handle) Name() ComponentName {
	if h == nil {
		return ""
	}
	return h.c.spec.Name
}

// Status returns the component's current status.
func (h *Handle) Status() Status {
	if h == nil {
		return Status{}
	}
	h.c.mu.Lock()
	defer h.c.mu.Unlock()
	return h.c.status.clone()
}

// Transition moves the component to another state allowed by its spec.
func (h *Handle) Transition(to StateName, opts ...TransitionOption) error {
	if h == nil {
		return nil
	}
	return h.ex.transition(h.c, to, false, opts)
}

// Update changes the reason or attributes without changing the state.
func (h *Handle) Update(opts ...TransitionOption) error {
	if h == nil {
		return nil
	}
	tc := transitionConfig{}
	for _, o := range opts {
		o(&tc)
	}
	e := h.ex
	e.mu.Lock()
	defer e.mu.Unlock()
	c := h.c
	c.mu.Lock()
	if c.removed {
		c.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrNotRegistered, c.spec.Name)
	}
	attrs, err := e.applyAttributes(c.status.Attributes, tc)
	if err != nil {
		c.mu.Unlock()
		return err
	}
	from := c.status.clone()
	c.status.Attributes = attrs
	if tc.reason != "" {
		c.status.Reason = tc.reason
	}
	to := c.status.clone()
	c.signalLocked()
	c.mu.Unlock()
	e.emitLocked(ControlEvent{Kind: EventUpdated, From: from, To: to})
	return nil
}

// BeginTransition starts a disruptive transition. It moves the component to
// `to` (which must be a PhaseTransitioning state), refuses new leases on the
// component, then waits until every lease taken earlier is released, at most
// until ctx is done or the drain timeout passes. It always returns a Done that
// the caller must call to finish the transition, unless err is non-nil.
//
// The caller must not hold locks that a lease holder may need while it waits.
func (h *Handle) BeginTransition(ctx context.Context, to StateName, opts ...TransitionOption) (Done, error) {
	if h == nil {
		return func(StateName, ...TransitionOption) error { return nil }, nil
	}
	e, c := h.ex, h.c
	if st, ok := c.spec.States[to]; !ok || st.Phase != PhaseTransitioning {
		return nil, fmt.Errorf("%w: %s: BeginTransition target %q must be a %s state", ErrInvalidTransition, c.spec.Name, to, PhaseTransitioning)
	}
	if err := e.transition(c, to, true, opts); err != nil {
		return nil, err
	}

	c.mu.Lock()
	var wait chan struct{}
	if c.leases > 0 {
		c.drained = make(chan struct{})
		wait = c.drained
	}
	c.mu.Unlock()

	if wait != nil {
		e.awaitDrain(ctx, c, wait)
	}

	var finished bool
	var mu sync.Mutex
	return func(final StateName, opts ...TransitionOption) error {
		mu.Lock()
		defer mu.Unlock()
		if finished {
			return fmt.Errorf("%w: %s: Done already called", ErrInvalidTransition, c.spec.Name)
		}
		if err := e.finishTransition(c, final, opts); err != nil {
			return err
		}
		finished = true
		return nil
	}, nil
}

func (e *Exchange) awaitDrain(ctx context.Context, c *component, wait chan struct{}) {
	start := e.cfg.clock.Now()
	timeout := make(chan struct{})
	t := e.cfg.clock.AfterFunc(e.cfg.drainTimeout, func() { close(timeout) })
	defer t.Stop()

	select {
	case <-wait:
		return
	case <-ctx.Done():
	case <-timeout:
	}

	c.mu.Lock()
	held := c.leases
	st := c.status.clone()
	c.mu.Unlock()
	if held == 0 {
		return
	}
	if e.disp != nil {
		e.disp.enqueue(notice{kind: noticeDrainTimeout, status: st, held: held, duration: e.cfg.clock.Now().Sub(start)})
	}
}

// Deregister removes the component. Subscribers receive an EventRemoved, gates
// that require it fail, and its watchdog stops.
func (h *Handle) Deregister() error {
	if h == nil {
		return nil
	}
	e, c := h.ex, h.c
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.comps[c.spec.Name] != c {
		return fmt.Errorf("%w: %s", ErrNotRegistered, c.spec.Name)
	}
	delete(e.comps, c.spec.Name)
	c.mu.Lock()
	c.removed = true
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
	c.token++
	from := c.status.clone()
	c.signalLocked()
	c.mu.Unlock()
	e.emitLocked(ControlEvent{Kind: EventRemoved, From: from})
	return nil
}

func (e *Exchange) transition(c *component, to StateName, begin bool, opts []TransitionOption) error {
	tc := transitionConfig{}
	for _, o := range opts {
		o(&tc)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	c.mu.Lock()
	if c.removed {
		c.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrNotRegistered, c.spec.Name)
	}
	if c.draining {
		c.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrTransitionInProgress, c.spec.Name)
	}
	ev, err := e.applyTransitionLocked(c, to, tc)
	if err != nil {
		c.mu.Unlock()
		e.reportInvalid(c.spec.Name, c.status.State, to, err)
		return err
	}
	if begin {
		c.draining = true
		c.version++
	}
	c.mu.Unlock()
	e.emitLocked(ev)
	return nil
}

func (e *Exchange) finishTransition(c *component, final StateName, opts []TransitionOption) error {
	tc := transitionConfig{}
	for _, o := range opts {
		o(&tc)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	c.mu.Lock()
	if c.removed {
		c.draining = false
		c.mu.Unlock()
		return nil
	}
	ev, err := e.applyTransitionLocked(c, final, tc)
	if err != nil {
		c.mu.Unlock()
		e.reportInvalid(c.spec.Name, c.status.State, final, err)
		return err
	}
	c.draining = false
	if c.drained != nil {
		close(c.drained)
		c.drained = nil
	}
	c.mu.Unlock()
	e.emitLocked(ev)
	return nil
}

// applyTransitionLocked validates and applies a state change. Both e.mu and
// c.mu must be held.
func (e *Exchange) applyTransitionLocked(c *component, to StateName, tc transitionConfig) (ControlEvent, error) {
	from := c.status
	if !c.spec.allows(from.State, to) {
		return ControlEvent{}, fmt.Errorf("%w: %s: %s -> %s", ErrInvalidTransition, c.spec.Name, from.State, to)
	}
	attrs, err := e.applyAttributes(from.Attributes, tc)
	if err != nil {
		return ControlEvent{}, err
	}
	next := c.spec.States[to]
	c.status = Status{
		Component:  c.spec.Name,
		State:      to,
		Phase:      next.Phase,
		Generation: from.Generation,
		Since:      e.cfg.clock.Now(),
		Reason:     tc.reason,
		Attributes: attrs,
	}
	if next.Phase == PhaseReady && from.Phase != PhaseReady && from.Phase != PhaseDegraded {
		c.status.Generation++
	}
	c.armTimerLocked(e)
	c.signalLocked()
	return ControlEvent{Kind: EventTransition, From: from.clone(), To: c.status.clone()}, nil
}

func (e *Exchange) reportInvalid(name ComponentName, from, to StateName, err error) {
	if e.disp != nil {
		e.disp.enqueue(notice{kind: noticeInvalid, component: name, from: from, to: to, err: err})
	}
}

// signalLocked records a change and wakes anything waiting for it.
func (c *component) signalLocked() {
	c.version++
	close(c.changed)
	c.changed = make(chan struct{})
}
