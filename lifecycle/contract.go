// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MPL-2.0

// Package lifecycle is a process-local registry of component lifecycles.
//
// Every Consul process (server, client agent, dataplane) creates one Exchange.
// Components register a state machine of their own states, report transitions,
// and other components read those states, subscribe to changes, or take short
// leases that a disruptive transition waits for before it proceeds.
//
// The Exchange is in-memory and ephemeral. It is unrelated to the replicated
// raft FSM: nothing here is persisted or shared between processes.
package lifecycle

import (
	"errors"
	"fmt"
	"time"
)

// SchemaVersion is the version of the ControlEvent contract. Changes to the
// contract are additive; the version is bumped when fields are added.
const SchemaVersion = 1

// ComponentName identifies a component within one Exchange. By convention it
// is "<entity>/<component>[/<instance>]", for example "server/state-store".
type ComponentName string

// StateName is a component-defined state, for example "Restoring".
type StateName string

// Phase is the coarse, shared meaning of a component state. Every state maps to
// exactly one phase so that other components can reason about any component
// without knowing its states. The set is deliberately closed.
type Phase int

const (
	PhaseUnknown Phase = iota
	// PhaseStarting means the component is not ready yet.
	PhaseStarting
	// PhaseReady means the component is fully serving.
	PhaseReady
	// PhaseDegraded means the component serves with reduced guarantees.
	PhaseDegraded
	// PhaseTransitioning means a disruptive change is in progress (restore,
	// reload, stop).
	PhaseTransitioning
	// PhaseFailed means the component cannot serve.
	PhaseFailed
	// PhaseStopped means the component was shut down.
	PhaseStopped
)

var phaseNames = map[Phase]string{
	PhaseUnknown:       "unknown",
	PhaseStarting:      "starting",
	PhaseReady:         "ready",
	PhaseDegraded:      "degraded",
	PhaseTransitioning: "transitioning",
	PhaseFailed:        "failed",
	PhaseStopped:       "stopped",
}

func (p Phase) String() string {
	if s, ok := phaseNames[p]; ok {
		return s
	}
	return fmt.Sprintf("phase(%d)", int(p))
}

func (p Phase) valid() bool {
	return p > PhaseUnknown && p <= PhaseStopped
}

// StateSpec describes one component-defined state.
type StateSpec struct {
	// Phase is the shared meaning of this state.
	Phase Phase
	// Next lists the states this state may transition to.
	Next []StateName
	// MaxDuration, when positive, reports the component as stuck if it stays
	// in this state longer than this.
	MaxDuration time.Duration
}

// ComponentSpec is the state machine a component registers.
type ComponentSpec struct {
	Name    ComponentName
	Initial StateName
	States  map[StateName]StateSpec
}

// Status is the current status of a component.
type Status struct {
	Component ComponentName
	State     StateName
	Phase     Phase
	// Generation increases each time the component becomes Ready from a phase
	// other than Ready or Degraded. A change tells listeners that anything tied
	// to the previous generation (cached data, open streams) is stale.
	Generation uint64
	Since      time.Time
	Reason     string
	// Attributes is small, component-defined metadata, for example the index
	// of a restored snapshot.
	Attributes map[string]string
}

func (s Status) clone() Status {
	if s.Attributes != nil {
		attrs := make(map[string]string, len(s.Attributes))
		for k, v := range s.Attributes {
			attrs[k] = v
		}
		s.Attributes = attrs
	}
	return s
}

// EventKind is the kind of a ControlEvent.
type EventKind int

const (
	// EventRegistered is emitted when a component registers. From is empty.
	EventRegistered EventKind = iota + 1
	// EventTransition is emitted on every state transition.
	EventTransition
	// EventUpdated is emitted when the reason or attributes change without a
	// state change.
	EventUpdated
	// EventRemoved is emitted when a component deregisters. To is empty.
	EventRemoved
)

func (k EventKind) String() string {
	switch k {
	case EventRegistered:
		return "registered"
	case EventTransition:
		return "transition"
	case EventUpdated:
		return "updated"
	case EventRemoved:
		return "removed"
	}
	return fmt.Sprintf("kind(%d)", int(k))
}

// ControlEvent is the contract for every lifecycle change.
type ControlEvent struct {
	SchemaVersion int
	// Seq is a per-Exchange sequence number, increasing by one per event.
	Seq  uint64
	Kind EventKind
	From Status
	To   Status
}

// Component returns the name of the component the event is about.
func (e ControlEvent) Component() ComponentName {
	if e.To.Component != "" {
		return e.To.Component
	}
	return e.From.Component
}

var (
	// ErrNotRegistered is returned when a component is not registered.
	ErrNotRegistered = errors.New("lifecycle: component not registered")
	// ErrAlreadyRegistered is returned when a component name is already in use.
	ErrAlreadyRegistered = errors.New("lifecycle: component already registered")
	// ErrInvalidTransition is returned for a transition the spec does not allow.
	ErrInvalidTransition = errors.New("lifecycle: invalid transition")
	// ErrInvalidSpec is returned for a ComponentSpec that fails validation.
	ErrInvalidSpec = errors.New("lifecycle: invalid component spec")
	// ErrTooManyComponents is returned when the component limit is reached.
	ErrTooManyComponents = errors.New("lifecycle: too many components")
	// ErrAttributesTooLarge is returned when attributes exceed the limits.
	ErrAttributesTooLarge = errors.New("lifecycle: attributes too large")
	// ErrTransitionInProgress is returned when a disruptive transition is
	// already in progress for the component.
	ErrTransitionInProgress = errors.New("lifecycle: transition already in progress")
	// ErrWaitMultiple is returned when Wait mode is used on a gate with more
	// than one component.
	ErrWaitMultiple = errors.New("lifecycle: Wait mode requires a single-component gate")
	// ErrClosed is returned after the Exchange has been closed.
	ErrClosed = errors.New("lifecycle: exchange closed")
)

// NotReadyError is returned when a gate requirement is not met.
type NotReadyError struct {
	Component ComponentName
	State     StateName
	Phase     Phase
	Reason    string
	// Requirement describes what was required.
	Requirement string

	err error
}

func (e *NotReadyError) Error() string {
	state := string(e.State)
	if state == "" {
		state = "absent"
	}
	msg := fmt.Sprintf("lifecycle: %s is %s (%s), requires %s", e.Component, state, e.Phase, e.Requirement)
	if e.Reason != "" {
		msg += ": " + e.Reason
	}
	return msg
}

// Unwrap returns ErrNotRegistered for components that are not registered.
func (e *NotReadyError) Unwrap() error { return e.err }

// IsNotReady reports whether err is, or wraps, a *NotReadyError.
func IsNotReady(err error) bool {
	var nr *NotReadyError
	return errors.As(err, &nr)
}
