package cache

import (
	"sync/atomic"
	"time"
)

// cacheEntry stores a single cache entry.
//
// Note that this isn't a very optimized structure currently. There are
// a lot of improvements that can be made here in the long term.
type cacheEntry struct {
	// Fields pertaining to the actual value
	Key   string
	Value interface{}
	Error error
	Index uint64

	// Metadata that is used for internal accounting
	Valid    bool          // True if the Value is set
	Fetching bool          // True if a fetch is already active
	Waiter   chan struct{} // Closed when this entry is invalidated

	// ExpiresRaw is the time.Time that this value expires. The time.Time
	// is immune to wall clock changes since we only use APIs that
	// operate on the monotonic value. The value is in an atomic.Value
	// so we have an efficient way to "touch" the value while maybe being
	// read without introducing complex locking.
	ExpiresRaw      atomic.Value
	ExpiresTTL      time.Duration
	ExpiryHeapIndex *int
}

// Expires is the time that this entry expires. The time.Time value returned
// has the monotonic clock preserved and should be used only with
// monotonic-safe operations to prevent wall clock changes affecting
// cache behavior.
func (e *cacheEntry) Expires() time.Time {
	return e.ExpiresRaw.Load().(time.Time)
}

// ResetExpires resets the expiration to be the ttl duration from now.
func (e *cacheEntry) ResetExpires() {
	e.ExpiresRaw.Store(time.Now().Add(e.ExpiresTTL))
}

// expiryHeap is a heap implementation that stores information about
// when entires expire. Implements container/heap.Interface.
//
// All operations on the heap and read/write of the heap contents require
// the proper entriesLock to be held on Cache.
type expiryHeap struct {
	Entries []*cacheEntry

	// NotifyCh is sent a value whenever the 0 index value of the heap
	// changes. This can be used to detect when the earliest value
	// changes.
	NotifyCh chan struct{}
}

func (h *expiryHeap) Len() int { return len(h.Entries) }

func (h *expiryHeap) Swap(i, j int) {
	h.Entries[i], h.Entries[j] = h.Entries[j], h.Entries[i]
	*h.Entries[i].ExpiryHeapIndex = i
	*h.Entries[j].ExpiryHeapIndex = j

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
	return h.Entries[i].Expires().Before(h.Entries[j].Expires())
}

func (h *expiryHeap) Push(x interface{}) {
	entry := x.(*cacheEntry)

	// For the first entry, we need to trigger a channel send because
	// Swap won't be called; nothing to swap! We can call it right away
	// because all heap operations are within a lock.
	if len(h.Entries) == 0 {
		*entry.ExpiryHeapIndex = 0 // Set correct initial index
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
