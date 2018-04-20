package cache

import (
	"time"
)

// cacheEntry stores a single cache entry.
//
// Note that this isn't a very optimized structure currently. There are
// a lot of improvements that can be made here in the long term.
type cacheEntry struct {
	// Fields pertaining to the actual value
	Value interface{}
	Error error
	Index uint64

	// Metadata that is used for internal accounting
	Valid    bool          // True if the Value is set
	Fetching bool          // True if a fetch is already active
	Waiter   chan struct{} // Closed when this entry is invalidated

	// Expiry contains information about the expiration of this
	// entry. This is a pointer as its shared as a value in the
	// expiryHeap as well.
	Expiry *cacheEntryExpiry
}

// cacheEntryExpiry contains the expiration information for a cache
// entry. Any modifications to this struct should be done only while
// the Cache entriesLock is held.
type cacheEntryExpiry struct {
	Key       string        // Key in the cache map
	Expires   time.Time     // Time when entry expires (monotonic clock)
	TTL       time.Duration // TTL for this entry to extend when resetting
	HeapIndex int           // Index in the heap
}

// Reset resets the expiration to be the ttl duration from now.
func (e *cacheEntryExpiry) Reset() {
	e.Expires = time.Now().Add(e.TTL)
}

// expiryHeap is a heap implementation that stores information about
// when entires expire. Implements container/heap.Interface.
//
// All operations on the heap and read/write of the heap contents require
// the proper entriesLock to be held on Cache.
type expiryHeap struct {
	Entries []*cacheEntryExpiry

	// NotifyCh is sent a value whenever the 0 index value of the heap
	// changes. This can be used to detect when the earliest value
	// changes.
	NotifyCh chan struct{}
}

func (h *expiryHeap) Len() int { return len(h.Entries) }

func (h *expiryHeap) Swap(i, j int) {
	h.Entries[i], h.Entries[j] = h.Entries[j], h.Entries[i]
	h.Entries[i].HeapIndex = i
	h.Entries[j].HeapIndex = j

	// If we're moving the 0 index, update the channel since we need
	// to re-update the timer we're waiting on for the soonest expiring
	// value.
	if i == 0 || j == 0 {
		h.NotifyCh <- struct{}{}
	}
}

func (h *expiryHeap) Less(i, j int) bool {
	// The usage of Before here is important (despite being obvious):
	// this function uses the monotonic time that should be available
	// on the time.Time value so the heap is immune to wall clock changes.
	return h.Entries[i].Expires.Before(h.Entries[j].Expires)
}

func (h *expiryHeap) Push(x interface{}) {
	entry := x.(*cacheEntryExpiry)

	// For the first entry, we need to trigger a channel send because
	// Swap won't be called; nothing to swap! We can call it right away
	// because all heap operations are within a lock.
	if len(h.Entries) == 0 {
		entry.HeapIndex = 0 // Set correct initial index
		h.NotifyCh <- struct{}{}
	}

	h.Entries = append(h.Entries, entry)
}

func (h *expiryHeap) Pop() interface{} {
	old := h.Entries
	n := len(old)
	x := old[n-1]
	h.Entries = old[0 : n-1]
	return x
}

func (h *expiryHeap) Notify() {
	h.NotifyCh <- struct{}{}
}
