package cache

import (
	"container/heap"
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
	//
	// There is a single edge case where the heap will not automatically
	// send a notification: if heap.Fix is called manually and the index
	// changed is 0 and the change doesn't result in any moves (stays at index
	// 0), then we won't detect the change. To work around this, please
	// always call the expiryHeap.Fix method instead.
	NotifyCh chan struct{}
}

// Identical to heap.Fix for this heap instance but will properly handle
// the edge case where idx == 0 and no heap modification is necessary,
// and still notify the NotifyCh.
//
// This is important for cache expiry since the expiry time may have been
// extended and if we don't send a message to the NotifyCh then we'll never
// reset the timer and the entry will be evicted early.
func (h *expiryHeap) Fix(entry *cacheEntryExpiry) {
	idx := entry.HeapIndex
	heap.Fix(h, idx)

	// This is the edge case we handle: if the prev (idx) and current (HeapIndex)
	// is zero, it means the head-of-line didn't change while the value
	// changed. Notify to reset our expiry worker.
	if idx == 0 && entry.HeapIndex == 0 {
		h.notify()
	}
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
		h.notify()
	}
}

func (h *expiryHeap) Less(i, j int) bool {
	// The usage of Before here is important (despite being obvious):
	// this function uses the monotonic time that should be available
	// on the time.Time value so the heap is immune to wall clock changes.
	return h.Entries[i].Expires.Before(h.Entries[j].Expires)
}

// heap.Interface, this isn't expected to be called directly.
func (h *expiryHeap) Push(x interface{}) {
	entry := x.(*cacheEntryExpiry)

	// Set initial heap index, if we're going to the end then Swap
	// won't be called so we need to initialize
	entry.HeapIndex = len(h.Entries)

	// For the first entry, we need to trigger a channel send because
	// Swap won't be called; nothing to swap! We can call it right away
	// because all heap operations are within a lock.
	if len(h.Entries) == 0 {
		h.notify()
	}

	h.Entries = append(h.Entries, entry)
}

// heap.Interface, this isn't expected to be called directly.
func (h *expiryHeap) Pop() interface{} {
	old := h.Entries
	n := len(old)
	x := old[n-1]
	h.Entries = old[0 : n-1]
	return x
}

func (h *expiryHeap) notify() {
	select {
	case h.NotifyCh <- struct{}{}:
		// Good

	default:
		// If the send would've blocked, we just ignore it. The reason this
		// is safe is because NotifyCh should always be a buffered channel.
		// If this blocks, it means that there is a pending message anyways
		// so the receiver will restart regardless.
	}
}
