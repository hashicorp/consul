// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: BUSL-1.1

// Package lifecycle tracks the lifecycle of a component of one process and lets
// work that depends on the component coordinate with disruptive changes to it.
//
// A Component is Starting, Ready, or Transitioning. Consumers take a short
// lease with Acquire while they use the component. The component's owner calls
// BeginTransition before it replaces whatever the consumers use: that refuses
// new leases and waits for the ones already held, so the replacement never
// overlaps a consumer that is mid-use.
//
// The state is in-memory and per process. It is unrelated to the replicated
// raft state machine: nothing here is persisted or shared between processes.
package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-metrics"
	"github.com/hashicorp/go-metrics/prometheus"
)

// DefaultDrainTimeout bounds how long BeginTransition waits for leases to be
// released.
const DefaultDrainTimeout = 30 * time.Second

// Phase is the coarse state of a component.
type Phase int

const (
	// PhaseStarting means the component is not ready yet.
	PhaseStarting Phase = iota + 1
	// PhaseReady means the component can be used; leases are granted.
	PhaseReady
	// PhaseTransitioning means the component is being replaced; leases are refused.
	PhaseTransitioning
)

func (p Phase) String() string {
	switch p {
	case PhaseStarting:
		return "starting"
	case PhaseReady:
		return "ready"
	case PhaseTransitioning:
		return "transitioning"
	}
	return fmt.Sprintf("phase(%d)", int(p))
}

// Status is a snapshot of a component.
type Status struct {
	Component string
	// State is the component's own name for its state, for example "Serving".
	State string
	Phase Phase
	// Generation increases each time the component becomes Ready again after a
	// transition. A change means whatever consumers cached from it is stale.
	Generation uint64
	Since      time.Time
	Reason     string
}

// NotReadyError is returned by Acquire when the component is not Ready.
type NotReadyError struct {
	Component string
	State     string
	Phase     Phase
	Reason    string
}

func (e *NotReadyError) Error() string {
	msg := fmt.Sprintf("%s is not ready: %s (%s)", e.Component, e.State, e.Phase)
	if e.Reason != "" {
		msg += ": " + e.Reason
	}
	return msg
}

// IsNotReady reports whether err is, or wraps, a *NotReadyError.
func IsNotReady(err error) bool {
	var nr *NotReadyError
	return errors.As(err, &nr)
}

// ErrTransitionInProgress is returned by BeginTransition when the component is
// already transitioning.
var ErrTransitionInProgress = errors.New("lifecycle: transition already in progress")

// Gauges are the gauges emitted by components. Names are relative to the
// metrics prefix.
var Gauges = []prometheus.GaugeDefinition{
	{
		Name: []string{"lifecycle", "phase"},
		Help: "Current phase of a lifecycle component: 1 starting, 2 ready, 3 transitioning.",
	},
}

// Counters are the counters emitted by components.
var Counters = []prometheus.CounterDefinition{
	{
		Name: []string{"lifecycle", "transition"},
		Help: "Counts phase changes of a lifecycle component.",
	},
	{
		Name: []string{"lifecycle", "drain_timeout"},
		Help: "Counts transitions that proceeded while leases were still held after the drain timeout.",
	},
}

// Option configures a Component.
type Option func(*Component)

// WithLogger sets the logger used for lifecycle changes.
func WithLogger(l hclog.Logger) Option {
	return func(c *Component) {
		if l != nil {
			c.logger = l
		}
	}
}

// WithDrainTimeout sets how long BeginTransition waits for leases.
func WithDrainTimeout(d time.Duration) Option {
	return func(c *Component) {
		if d > 0 {
			c.drainTimeout = d
		}
	}
}

// Component is the lifecycle of one component. A nil *Component is valid and
// behaves as an always-ready component with no state: every method is a no-op.
type Component struct {
	name         string
	logger       hclog.Logger
	drainTimeout time.Duration

	mu      sync.Mutex
	status  Status
	leases  int
	drained chan struct{} // closed when the last lease is released during a drain
}

// NewComponent returns a component in PhaseStarting.
func NewComponent(name string, opts ...Option) *Component {
	c := &Component{
		name:         name,
		logger:       hclog.NewNullLogger(),
		drainTimeout: DefaultDrainTimeout,
		status:       Status{Component: name, State: "Starting", Phase: PhaseStarting, Since: time.Now()},
	}
	for _, o := range opts {
		o(c)
	}
	c.emit(Status{}, c.status)
	return c
}

// Status returns the current status.
func (c *Component) Status() Status {
	if c == nil {
		return Status{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.status
}

// Ready moves a starting component to PhaseReady in the given state. It has no
// effect on a component that is not Starting.
func (c *Component) Ready(state, reason string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	if c.status.Phase != PhaseStarting {
		c.mu.Unlock()
		return
	}
	from := c.status
	c.status = Status{Component: c.name, State: state, Phase: PhaseReady, Generation: from.Generation + 1, Since: time.Now(), Reason: reason}
	to := c.status
	c.mu.Unlock()
	c.emit(from, to)
}

// Acquire takes a lease on a Ready component and returns the function that
// releases it. It never blocks, so it is safe to call while holding other
// locks. While the component is not Ready it returns a *NotReadyError.
//
// Keep leases short; never hold one for the lifetime of a stream. Release is
// safe to call more than once.
func (c *Component) Acquire() (release func(), err error) {
	if c == nil {
		return func() {}, nil
	}
	c.mu.Lock()
	if c.status.Phase != PhaseReady {
		st := c.status
		c.mu.Unlock()
		return nil, &NotReadyError{Component: st.Component, State: st.State, Phase: st.Phase, Reason: st.Reason}
	}
	c.leases++
	c.mu.Unlock()

	var once sync.Once
	return func() { once.Do(c.release) }, nil
}

func (c *Component) release() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.leases > 0 {
		c.leases--
	}
	if c.leases == 0 && c.drained != nil {
		close(c.drained)
		c.drained = nil
	}
}

// BeginTransition starts a disruptive change. It moves the component to
// PhaseTransitioning, so new leases are refused, then waits until the leases
// already held are released, for at most the drain timeout or until ctx is
// done. After a timeout it logs a warning and proceeds.
//
// The caller must not hold locks that a lease holder may need while it waits.
// finish must be called exactly once when the change is done; it makes the
// component Ready again in the given state and increases the generation.
func (c *Component) BeginTransition(ctx context.Context, state, reason string) (finish func(state, reason string), err error) {
	if c == nil {
		return func(string, string) {}, nil
	}

	c.mu.Lock()
	if c.status.Phase == PhaseTransitioning {
		c.mu.Unlock()
		return nil, fmt.Errorf("%w: %s", ErrTransitionInProgress, c.name)
	}
	from := c.status
	c.status = Status{Component: c.name, State: state, Phase: PhaseTransitioning, Generation: from.Generation, Since: time.Now(), Reason: reason}
	to := c.status
	var drained chan struct{}
	if c.leases > 0 {
		drained = make(chan struct{})
		c.drained = drained
	}
	c.mu.Unlock()
	c.emit(from, to)

	if drained != nil {
		c.awaitDrain(ctx, drained)
	}

	var once sync.Once
	return func(state, reason string) {
		once.Do(func() { c.finish(state, reason) })
	}, nil
}

func (c *Component) awaitDrain(ctx context.Context, drained <-chan struct{}) {
	start := time.Now()
	timer := time.NewTimer(c.drainTimeout)
	defer timer.Stop()
	select {
	case <-drained:
		return
	case <-ctx.Done():
	case <-timer.C:
	}

	c.mu.Lock()
	held := c.leases
	c.mu.Unlock()
	if held == 0 {
		return
	}
	c.logger.Warn("lifecycle transition proceeding with leases still held",
		"component", c.name, "leases", held, "waited", time.Since(start))
	metrics.IncrCounterWithLabels([]string{"lifecycle", "drain_timeout"}, 1, c.labels())
}

func (c *Component) finish(state, reason string) {
	c.mu.Lock()
	from := c.status
	c.status = Status{Component: c.name, State: state, Phase: PhaseReady, Generation: from.Generation + 1, Since: time.Now(), Reason: reason}
	to := c.status
	c.drained = nil
	c.mu.Unlock()
	c.emit(from, to)
}

func (c *Component) labels() []metrics.Label {
	return []metrics.Label{{Name: "component", Value: c.name}}
}

// emit logs and records a phase change. It is called without c.mu held.
func (c *Component) emit(from, to Status) {
	metrics.SetGaugeWithLabels([]string{"lifecycle", "phase"}, float32(to.Phase), c.labels())
	if from.Phase != 0 {
		metrics.IncrCounterWithLabels([]string{"lifecycle", "transition"}, 1, []metrics.Label{
			{Name: "component", Value: c.name},
			{Name: "from", Value: from.Phase.String()},
			{Name: "to", Value: to.Phase.String()},
		})
	}

	args := []interface{}{"component", c.name, "state", to.State, "phase", to.Phase.String(), "generation", to.Generation}
	if to.Reason != "" {
		args = append(args, "reason", to.Reason)
	}
	if from.Phase == PhaseTransitioning || to.Phase == PhaseTransitioning {
		c.logger.Info("lifecycle transition", args...)
		return
	}
	c.logger.Debug("lifecycle state", args...)
}
