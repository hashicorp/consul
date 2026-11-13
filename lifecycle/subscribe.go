// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MPL-2.0

package lifecycle

import (
	"context"
	"sync"
)

// UpdateKind is the kind of an Update delivered to a Subscription.
type UpdateKind int

const (
	// UpdateSnapshot carries the current status of every matching component.
	UpdateSnapshot UpdateKind = iota + 1
	// UpdateEvent carries one ControlEvent.
	UpdateEvent
)

// Update is one item read from a Subscription.
type Update struct {
	Kind UpdateKind
	// Seq is the event sequence number; for a snapshot it is the sequence of
	// the last event included in it. Later events have a larger Seq.
	Seq uint64
	// Resync is true when the snapshot replaces events the subscriber was too
	// slow to read.
	Resync   bool
	Statuses []Status
	Event    ControlEvent
}

// Subscription delivers a snapshot of the current statuses, then every
// matching ControlEvent in order. A slow reader never blocks writers: when its
// buffer is full the queued events are dropped and the next read returns a
// fresh snapshot with Resync set. Because statuses are level-based, skipping
// intermediate events is safe.
type Subscription struct {
	ex  *Exchange
	cfg subscribeConfig

	mu       sync.Mutex
	queue    []Update
	overflow bool
	closed   bool
	notify   chan struct{}
	stop     func() bool
}

// Subscribe starts a subscription. It ends when ctx is done, Close is called or
// the Exchange is closed. A nil Exchange returns a subscription that only ends.
func (e *Exchange) Subscribe(ctx context.Context, opts ...SubscribeOption) (*Subscription, error) {
	cfg := subscribeConfig{buffer: DefaultSubscriptionBuffer}
	for _, o := range opts {
		o(&cfg)
	}
	s := &Subscription{ex: e, cfg: cfg, notify: make(chan struct{}, 1)}
	if e == nil {
		s.stop = context.AfterFunc(ctx, s.markClosed)
		return s, nil
	}

	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return nil, ErrClosed
	}
	s.queue = []Update{{Kind: UpdateSnapshot, Seq: e.seq, Statuses: e.statusesLocked(cfg)}}
	e.subs[s] = struct{}{}
	e.mu.Unlock()

	s.stop = context.AfterFunc(ctx, s.Close)
	return s, nil
}

// Next returns the next update, blocking until one is available, ctx is done or
// the subscription is closed (ErrClosed).
func (s *Subscription) Next(ctx context.Context) (Update, error) {
	for {
		s.mu.Lock()
		if s.overflow {
			s.mu.Unlock()
			if u, ok := s.resync(); ok {
				return u, nil
			}
			continue
		}
		if len(s.queue) > 0 {
			u := s.queue[0]
			s.queue[0] = Update{}
			s.queue = s.queue[1:]
			s.mu.Unlock()
			return u, nil
		}
		if s.closed {
			s.mu.Unlock()
			return Update{}, ErrClosed
		}
		s.mu.Unlock()

		select {
		case <-ctx.Done():
			return Update{}, ctx.Err()
		case <-s.notify:
		}
	}
}

// Close ends the subscription. It is safe to call more than once.
func (s *Subscription) Close() {
	if s.ex != nil {
		s.ex.mu.Lock()
		delete(s.ex.subs, s)
		s.ex.mu.Unlock()
	}
	s.markClosed()
	if s.stop != nil {
		s.stop()
	}
}

func (s *Subscription) markClosed() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	s.signal()
}

func (s *Subscription) signal() {
	select {
	case s.notify <- struct{}{}:
	default:
	}
}

// push queues an event. Called with Exchange.mu held for writing.
func (s *Subscription) push(ev ControlEvent) {
	if !s.cfg.matches(ev.Component()) {
		return
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	if s.overflow {
		s.mu.Unlock()
		return
	}
	if len(s.queue) >= s.cfg.buffer {
		s.queue = nil
		s.overflow = true
	} else {
		s.queue = append(s.queue, Update{Kind: UpdateEvent, Seq: ev.Seq, Event: ev})
	}
	s.mu.Unlock()
	s.signal()
}

// resync replaces the dropped events with a fresh snapshot. Holding the
// Exchange read lock means no event can be emitted while the snapshot is built,
// so every later event has a larger Seq than the snapshot.
func (s *Subscription) resync() (Update, bool) {
	e := s.ex
	e.mu.RLock()
	defer e.mu.RUnlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.overflow {
		return Update{}, false
	}
	s.overflow = false
	s.queue = nil
	return Update{Kind: UpdateSnapshot, Seq: e.seq, Resync: true, Statuses: e.statusesLocked(s.cfg)}, true
}

// WaitFor blocks until the component is registered and in one of the phases,
// or ctx is done.
func (e *Exchange) WaitFor(ctx context.Context, name ComponentName, phases ...Phase) (Status, error) {
	if e == nil {
		return Status{}, nil
	}
	req := Requirement{Component: name, Phases: phases}
	for {
		e.mu.RLock()
		c := e.comps[name]
		regCh := e.regCh
		e.mu.RUnlock()
		if c == nil {
			select {
			case <-ctx.Done():
				return Status{}, ctx.Err()
			case <-regCh:
				continue
			}
		}
		c.mu.Lock()
		st := c.status.clone()
		removed := c.removed
		changed := c.changed
		c.mu.Unlock()
		if !removed && req.satisfiedBy(st) {
			return st, nil
		}
		select {
		case <-ctx.Done():
			return st, ctx.Err()
		case <-changed:
		}
	}
}
