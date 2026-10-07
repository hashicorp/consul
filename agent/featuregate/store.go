// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package featuregate

import (
	"sync"
	"sync/atomic"

	"github.com/hashicorp/go-memdb"
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

	// featureMu guards per-feature change tracking used by ObserveForQuery, so
	// that a query depending on one feature is not woken (and its index is not
	// advanced) when an unrelated feature, or an unchanged value, is published.
	featureMu          sync.Mutex
	featureWatchCh     map[string]chan struct{}
	featureChangeIndex map[string]uint64
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
			s.recordFeatureChanges(current, next, next.StatusIndex)
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
//
// observedIndex is recorded as the change index of every feature that Reset
// turns off, so blocking queries using ObserveForQuery see a strictly newer
// index and deliver the disabled result. Pass the Raft applied index: it
// advances on every FSM replacement and is never lower than an index already
// returned to a query.
func (s *Store) Reset(observedIndex uint64) {
	for {
		current := s.snapshot.Load()
		if current == nil {
			return
		}
		if s.snapshot.CompareAndSwap(current, nil) {
			s.recordFeatureChanges(current, nil, observedIndex)
			s.notifyWatchers()
			return
		}
	}
}

// Replace installs snapshot unconditionally, even when its StatusIndex is not
// newer than the cached generation. It must be used instead of Publish when
// the backing FSM state store has been replaced (e.g. a snapshot restore),
// because the restored status can be older than the one cached from the
// abandoned store and Publish would reject it, keeping the pre-restore
// decision active. Features whose effective value changes are recorded at an
// index newer than observedIndex (pass the Raft applied index), so blocking
// queries using ObserveForQuery deliver the restored decision; features whose
// value is unchanged do not wake.
func (s *Store) Replace(snapshot Snapshot, observedIndex uint64) {
	next := snapshot.clone()
	current := s.snapshot.Swap(next)
	s.recordFeatureChanges(current, next, max(next.StatusIndex, observedIndex))
	s.notifyWatchers()
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

// ObserveForQuery lets a memdb blocking query depend on one feature gate. It
// registers a channel on ws (when ws is non-nil) that fires only when the
// feature's effective value changes, and returns whether the feature is enabled
// together with max(index, the StatusIndex at which the feature last changed).
// A value change is therefore delivered as a newer index even when the queried
// table did not change, while publishes that leave this feature unchanged
// neither wake the query nor advance its index. A nil or uninitialized store is
// disabled.
func (s *Store) ObserveForQuery(ws memdb.WatchSet, feature Feature, index uint64) (bool, uint64) {
	if s == nil {
		return false, index
	}

	// Register the watch before loading so a concurrent Publish is never missed:
	// the channel is closed only after the new snapshot is installed.
	s.featureMu.Lock()
	if ws != nil {
		if s.featureWatchCh == nil {
			s.featureWatchCh = make(map[string]chan struct{})
		}
		ch, ok := s.featureWatchCh[feature.name]
		if !ok {
			ch = make(chan struct{})
			s.featureWatchCh[feature.name] = ch
		}
		ws.Add(ch)
	}
	changeIndex := s.featureChangeIndex[feature.name]
	s.featureMu.Unlock()

	return s.Enabled(feature), max(index, changeIndex)
}

// recordFeatureChanges notifies per-feature watchers for every feature whose
// effective value differs between prev and next (nil means all disabled), and
// records each change at a query index strictly newer than the feature's
// previous change: max(statusIndex, previous+1).
func (s *Store) recordFeatureChanges(prev, next *Snapshot, statusIndex uint64) {
	var prevFeatures, nextFeatures map[string]bool
	if prev != nil {
		prevFeatures = prev.Features
	}
	if next != nil {
		nextFeatures = next.Features
	}

	s.featureMu.Lock()
	defer s.featureMu.Unlock()

	changed := func(name string) {
		if s.featureChangeIndex == nil {
			s.featureChangeIndex = make(map[string]uint64)
		}
		// Every effective transition must be strictly newer than the last one,
		// or a blocking query that already returned that index would re-block on
		// the changed value. A status index can be older than the recorded one
		// only after Reset recorded the Raft applied index and a later restore
		// installed an older status; that restore already advanced Raft past the
		// reset index, so prev+1 never runs ahead of real Raft indexes.
		s.featureChangeIndex[name] = max(statusIndex, s.featureChangeIndex[name]+1)
		if ch, ok := s.featureWatchCh[name]; ok {
			close(ch)
			delete(s.featureWatchCh, name)
		}
	}
	for name, enabled := range nextFeatures {
		if prevFeatures[name] != enabled {
			changed(name)
		}
	}
	for name, enabled := range prevFeatures {
		if _, ok := nextFeatures[name]; !ok && enabled {
			changed(name)
		}
	}
}
