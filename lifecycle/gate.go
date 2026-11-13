// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MPL-2.0

package lifecycle

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Requirement is one condition of a Gate, declared by the consumer.
type Requirement struct {
	Component ComponentName
	// Phases accepted for the component. Ignored when Check is set.
	Phases []Phase
	// Name describes a custom Check in errors and logs.
	Name string
	// Check is an optional custom predicate. It must be pure and fast; it runs
	// on a copy of the status with no lock held by this package.
	Check func(Status) bool
}

func (r Requirement) satisfiedBy(s Status) bool {
	if r.Check != nil {
		return r.Check(s)
	}
	for _, p := range r.Phases {
		if s.Phase == p {
			return true
		}
	}
	return false
}

func (r Requirement) String() string {
	if r.Check != nil {
		if r.Name != "" {
			return r.Name
		}
		return "custom check"
	}
	parts := make([]string, len(r.Phases))
	for i, p := range r.Phases {
		parts[i] = p.String()
	}
	return "phase in [" + strings.Join(parts, ", ") + "]"
}

// Gate is a set of requirements a consumer declares at the call site. It is a
// value; building one is cheap and does not touch the Exchange.
type Gate struct {
	ex   *Exchange
	reqs []Requirement
}

// Require returns a gate that requires the component to be in one of phases.
func (e *Exchange) Require(name ComponentName, phases ...Phase) Gate {
	return Gate{ex: e}.And(name, phases...)
}

// RequireFunc returns a gate with a custom predicate on the component status.
func (e *Exchange) RequireFunc(name ComponentName, description string, check func(Status) bool) Gate {
	return Gate{ex: e}.AndFunc(name, description, check)
}

// And adds a phase requirement.
func (g Gate) And(name ComponentName, phases ...Phase) Gate {
	return g.with(Requirement{Component: name, Phases: append([]Phase(nil), phases...)})
}

// AndFunc adds a custom predicate requirement.
func (g Gate) AndFunc(name ComponentName, description string, check func(Status) bool) Gate {
	return g.with(Requirement{Component: name, Name: description, Check: check})
}

func (g Gate) with(r Requirement) Gate {
	reqs := make([]Requirement, len(g.reqs), len(g.reqs)+1)
	copy(reqs, g.reqs)
	return Gate{ex: g.ex, reqs: append(reqs, r)}
}

// AcquireMode controls what Acquire does when the gate is closed.
type AcquireMode int

const (
	// FailFast returns a *NotReadyError immediately. It never blocks, so it is
	// safe to call while holding other locks.
	FailFast AcquireMode = iota
	// Wait blocks until the gate opens or ctx is done. Only allowed for gates on
	// a single component, and never while holding other locks.
	Wait
)

// Lease marks in-flight work that depends on the gate's components. A
// BeginTransition on any of them waits for it to be released. Keep leases
// short; never hold one for the lifetime of a stream.
type Lease struct {
	comps []*component
	once  sync.Once
}

// Release ends the lease. It is safe to call more than once and on nil.
func (l *Lease) Release() {
	if l == nil {
		return
	}
	l.once.Do(func() {
		for _, c := range l.comps {
			c.release()
		}
	})
}

func (c *component) release() {
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

type gateGroup struct {
	name ComponentName
	reqs []Requirement
}

func (g Gate) groups() []gateGroup {
	byName := map[ComponentName][]Requirement{}
	for _, r := range g.reqs {
		byName[r.Component] = append(byName[r.Component], r)
	}
	out := make([]gateGroup, 0, len(byName))
	for name, reqs := range byName {
		out = append(out, gateGroup{name: name, reqs: reqs})
	}
	// A fixed order for multi-component leases avoids lock-order surprises.
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

// Check reports whether the gate is open right now, without taking a lease.
func (g Gate) Check() error {
	if g.ex == nil {
		return nil
	}
	for _, grp := range g.groups() {
		c := g.ex.lookup(grp.name)
		if c == nil {
			return notRegistered(grp.name)
		}
		c.mu.Lock()
		st, removed, draining := c.status.clone(), c.removed, c.draining
		c.mu.Unlock()
		if err := evaluate(grp, st, removed, draining); err != nil {
			return err
		}
	}
	return nil
}

// Acquire takes a lease if the gate is open. In FailFast mode a closed gate
// returns a *NotReadyError. A nil Exchange always succeeds.
func (g Gate) Acquire(ctx context.Context, mode AcquireMode) (*Lease, error) {
	if g.ex == nil {
		return &Lease{}, nil
	}
	groups := g.groups()
	if mode == Wait && len(groups) != 1 {
		return nil, ErrWaitMultiple
	}
	for {
		lease, changed, err := g.tryAcquire(groups)
		if err == nil || mode == FailFast {
			return lease, err
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("%w: %v", ctx.Err(), err)
		case <-changed:
		}
	}
}

func (g Gate) tryAcquire(groups []gateGroup) (*Lease, <-chan struct{}, error) {
	lease := &Lease{}
	for _, grp := range groups {
		g.ex.mu.RLock()
		c := g.ex.comps[grp.name]
		regCh := g.ex.regCh
		g.ex.mu.RUnlock()
		if c == nil {
			lease.Release()
			return nil, regCh, notRegistered(grp.name)
		}

		// Predicates run on a copy, with no lock held.
		c.mu.Lock()
		st, removed, draining, changed, version := c.status.clone(), c.removed, c.draining, c.changed, c.version
		c.mu.Unlock()
		if err := evaluate(grp, st, removed, draining); err != nil {
			lease.Release()
			return nil, changed, err
		}

		c.mu.Lock()
		if c.version != version {
			// Changed between the check and the lease; let the caller retry.
			changed = c.changed
			c.mu.Unlock()
			lease.Release()
			return nil, changed, &NotReadyError{Component: grp.name, State: st.State, Phase: st.Phase, Reason: "changed while acquiring", Requirement: requirementString(grp.reqs)}
		}
		c.leases++
		c.mu.Unlock()
		lease.comps = append(lease.comps, c)
	}
	return lease, nil, nil
}

func evaluate(grp gateGroup, st Status, removed, draining bool) error {
	if removed {
		return notRegistered(grp.name)
	}
	if draining {
		return &NotReadyError{Component: grp.name, State: st.State, Phase: st.Phase, Reason: "transition in progress", Requirement: requirementString(grp.reqs)}
	}
	for _, r := range grp.reqs {
		if !r.satisfiedBy(st) {
			return &NotReadyError{Component: grp.name, State: st.State, Phase: st.Phase, Reason: st.Reason, Requirement: r.String()}
		}
	}
	return nil
}

func requirementString(reqs []Requirement) string {
	parts := make([]string, len(reqs))
	for i, r := range reqs {
		parts[i] = r.String()
	}
	return strings.Join(parts, " and ")
}

func notRegistered(name ComponentName) error {
	return &NotReadyError{Component: name, Reason: "not registered", Requirement: "registered", err: ErrNotRegistered}
}
