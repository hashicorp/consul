// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package featuregate

import (
	"sync"
	"sync/atomic"
)

// Gate is the minimal runtime interface used at behavior boundaries.
type Gate interface {
	Enabled(feature Feature) bool
}

// WatchableGate is implemented by caches that can notify long-lived runtime
// consumers when a committed effective decision changes.
type WatchableGate interface {
	Gate
	Watch() <-chan struct{}
}

// Snapshot is the immutable local projection of one committed resolved-status
// generation. Features contains final EffectiveEnabled values only.
type Snapshot struct {
	StatusIndex    uint64
	PolicyIndex    uint64
	RegistryDigest string
	Features       map[string]bool
}

func (s Snapshot) clone() *Snapshot {
	clone := s
	if s.Features != nil {
		clone.Features = make(map[string]bool, len(s.Features))
		for name, enabled := range s.Features {
			clone.Features[name] = enabled
		}
	}
	return &clone
}

// Store publishes whole immutable snapshots using an atomic pointer. The zero
// value is ready for use and fails closed.
type Store struct {
	snapshot atomic.Pointer[Snapshot]

	watchMu sync.Mutex
	watchCh chan struct{}
}

var _ Gate = (*Store)(nil)
var _ WatchableGate = (*Store)(nil)

// Publish atomically installs snapshot only when it is newer than the current
// committed generation.
func (s *Store) Publish(snapshot Snapshot) bool {
	next := snapshot.clone()
	for {
		current := s.snapshot.Load()
		if current != nil && next.StatusIndex <= current.StatusIndex {
			return false
		}
		if s.snapshot.CompareAndSwap(current, next) {
			s.notifyWatchers()
			return true
		}
	}
}

// Watch returns a channel that is closed after the next successful Publish.
// Callers must obtain a new channel after every notification.
//
// Watch is a generation-change watch: it fires whenever a newer StatusIndex is
// committed, even if the effective feature values in the Features map are
// unchanged. Consumers that only care about value changes (e.g. the API Gateway
// handler) should compare the relevant feature values themselves after waking up
// and short-circuit if nothing has changed. This design keeps Watch simple and
// fully extensible: new callers can react to any generation advance without
// requiring the store to understand every consumer's equality semantics.
func (s *Store) Watch() <-chan struct{} {
	s.watchMu.Lock()
	defer s.watchMu.Unlock()
	if s.watchCh == nil {
		s.watchCh = make(chan struct{})
	}
	return s.watchCh
}

func (s *Store) notifyWatchers() {
	s.watchMu.Lock()
	defer s.watchMu.Unlock()
	if s.watchCh != nil {
		close(s.watchCh)
	}
	s.watchCh = make(chan struct{})
}

// Reset clears the installed snapshot, returning the store to an
// uninitialized, fail-closed state. It notifies any active Watch callers so
// that they re-evaluate the gate. Reset must be called when the backing FSM
// state store is abandoned (e.g. snapshot restore) and the replacement store
// has not yet supplied a valid committed status, preventing a stale generation
// from surviving across FSM replacements.
func (s *Store) Reset() {
	for {
		current := s.snapshot.Load()
		if current == nil {
			return
		}
		if s.snapshot.CompareAndSwap(current, nil) {
			s.notifyWatchers()
			return
		}
	}
}

// Enabled returns the final cached decision. Missing/unknown features and an
// uninitialized store are disabled.
func (s *Store) Enabled(feature Feature) bool {
	current := s.snapshot.Load()
	return current != nil && current.Features[feature.name]
}

// Current returns a defensive copy for diagnostics and tests.
func (s *Store) Current() Snapshot {
	current := s.snapshot.Load()
	if current == nil {
		return Snapshot{}
	}
	return *current.clone()
}
