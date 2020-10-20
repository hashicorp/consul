package cache

import (
	"container/heap"
	"time"
)

// cacheEntryExpiry contains the expiration time for a cache entry.
type cacheEntryExpiry struct {
	Key       string    // Key in the cache map
	Expires   time.Time // Time when entry expires (monotonic clock)
	HeapIndex int       // Index in the heap
}

// TODO: use or remove
func newCacheEntry(key string, expiry time.Duration) *cacheEntryExpiry {
	return &cacheEntryExpiry{Key: key, Expires: time.Now().Add(expiry)}
}

// expiryHeap is a container/heap.Interface implementation that expires entries
// in the cache when their expiration time is reached.
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

func (h *expiryHeap) Add(key string, expiry time.Duration) *cacheEntryExpiry {
	entry := &cacheEntryExpiry{Key: key, Expires: time.Now().Add(expiry)}
	heap.Push(h, entry)
	return entry
}

// Update the entry that is currently at idx with the new expiry time. The heap
// will be rebalanced after the entry is updated.
//
// Must be synchronized by the caller.
func (h *expiryHeap) Update(idx int, expiry time.Duration) {
	entry := h.Entries[idx]
	entry.Expires = time.Now().Add(expiry)
	heap.Fix(h, idx)

	// If the previous index and current index are both zero then Fix did not
	// swap the entry, and notify must be called here.
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

	// Set the initial heap index to the last index. If the entry is swapped it
	// will have the correct set, and if it remains at the end the last index will
	// be correct.
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
	n := len(h.Entries)
	entries := h.Entries
	last := entries[n-1]
	h.Entries = entries[0 : n-1]
	return last
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
