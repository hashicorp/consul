package watch

import "context"

// Map safely stores and retrieves values by validating that
// there is a live watch for a key. InitWatch must be called
// to associate a key with its cancel function before any
// Set's are called.
type Map[K comparable, V any] struct {
	M map[K]watchedVal[V]
}

type watchedVal[V any] struct {
	Val *V

	// keeping cancel private has a beneficial side effect:
	// copying Map with copystructure.Copy will zero out
	// cancel, preventing it from being called by the
	// receiver of a proxy config snapshot.
	cancel context.CancelFunc
}

func NewMap[K comparable, V any]() Map[K, V] {
	return Map[K, V]{M: make(map[K]watchedVal[V])}
}

// InitWatch associates a cancel function with a key,
// allowing Set to be called for the key. The cancel
// function is allowed to be nil.
//
// Any existing data for a key will be cancelled and
// overwritten.
func (m Map[K, V]) InitWatch(key K, cancel func()) {
	if _, present := m.M[key]; present {
		m.CancelWatch(key)
	}
	m.M[key] = watchedVal[V]{
		cancel: cancel,
	}
}

// CancelWatch first calls the cancel function
// associated with the key then deletes the key
// from the map. No-op if key is not present.
func (m Map[K, V]) CancelWatch(key K) {
	if entry, ok := m.M[key]; ok {
		if entry.cancel != nil {
			entry.cancel()
		}
		delete(m.M, key)
	}
}

// IsWatched returns true if InitWatch has been
// called for key and has not been cancelled by
// CancelWatch.
func (m Map[K, V]) IsWatched(key K) bool {
	if _, present := m.M[key]; present {
		return true
	}
	return false
}

// Set stores V if K exists in the map.
// No-op if the key never was initialized with InitWatch
// or if the entry got cancelled by CancelWatch.
func (m Map[K, V]) Set(key K, val V) bool {
	if entry, ok := m.M[key]; ok {
		entry.Val = &val
		m.M[key] = entry
		return true
	}
	return false
}

// Get returns the underlying value for a key.
// If an entry has been set, returns (V, true).
// Otherwise, returns the zero value (V, false).
//
// Note that even if InitWatch has been called
// for a key, unless Set has been called this
// function will return false.
func (m Map[K, V]) Get(key K) (V, bool) {
	if entry, ok := m.M[key]; ok {
		if entry.Val != nil {
			return *entry.Val, true
		}
	}
	var empty V
	return empty, false
}

func (m Map[K, V]) Len() int {
	return len(m.M)
}

// ForEachKey iterates through the map, calling f
// for each iteration. It is up to the caller to
// Get the value and nil-check if required.
// Stops iterating if f returns false.
// Order of iteration is non-deterministic.
func (m Map[K, V]) ForEachKey(f func(K) bool) {
	for k := range m.M {
		if ok := f(k); !ok {
			return
		}
	}
}
