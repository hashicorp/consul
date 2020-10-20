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
	NotifyCh chan struct{}
}

// Initialize the heap. The buffer of 1 is really important because
// its possible for the expiry loop to trigger the heap to update
// itself and it'd block forever otherwise.
func newExpiryHeap() *expiryHeap {
	h := &expiryHeap{NotifyCh: make(chan struct{}, 1)}
	heap.Init(h)
	return h
}

// Add an entry to the heap.
//
// Must be synchronized by the caller.
func (h *expiryHeap) Add(key string, expiry time.Duration) *cacheEntryExpiry {
	entry := &cacheEntryExpiry{
		Key:     key,
		Expires: time.Now().Add(expiry),
		// Set the initial heap index to the last index. If the entry is swapped it
		// will have the correct index set, and if it remains at the end the last
		// index will be correct.
		HeapIndex: len(h.Entries),
	}
	heap.Push(h, entry)
	if entry.HeapIndex == 0 {
		h.notify()
	}
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
	if idx == 0 || entry.HeapIndex == 0 {
		h.notify()
	}
}

// Must be synchronized by the caller.
func (h *expiryHeap) Remove(idx int) {
	entry := h.Entries[idx]
	heap.Remove(h, idx)

	// A goroutine which is fetching a new value will have a reference to this
	// entry. When it re-acquires the lock it needs to be informed that
	// the entry was expired while it was fetching. Setting HeapIndex to -1
	// indicates that the entry is no longer in the heap, and must be re-added.
	entry.HeapIndex = -1

	if idx == 0 {
		h.notify()
	}
}

func (h *expiryHeap) Len() int { return len(h.Entries) }

func (h *expiryHeap) Swap(i, j int) {
	h.Entries[i], h.Entries[j] = h.Entries[j], h.Entries[i]
	h.Entries[i].HeapIndex = i
	h.Entries[j].HeapIndex = j
}

func (h *expiryHeap) Less(i, j int) bool {
	// The usage of Before here is important (despite being obvious):
	// this function uses the monotonic time that should be available
	// on the time.Time value so the heap is immune to wall clock changes.
	return h.Entries[i].Expires.Before(h.Entries[j].Expires)
}

// heap.Interface, this isn't expected to be called directly.
func (h *expiryHeap) Push(x interface{}) {
	h.Entries = append(h.Entries, x.(*cacheEntryExpiry))
}

// heap.Interface, this isn't expected to be called directly.
func (h *expiryHeap) Pop() interface{} {
	n := len(h.Entries)
	entries := h.Entries
	last := entries[n-1]
	h.Entries = entries[0 : n-1]
	return last
}

// notify the timer that the head value has changed, so the expiry time has
// also likely changed.
func (h *expiryHeap) notify() {
	// Send to channel without blocking. Skips sending if there is already
	// an item in the buffered channel.
	select {
	case h.NotifyCh <- struct{}{}:
	default:
	}
}

// Must be synchronized by the caller.
func (h *expiryHeap) Next() timer {
	if len(h.Entries) == 0 {
		return timer{}
	}
	entry := h.Entries[0]
	return timer{
		timer: time.NewTimer(time.Until(entry.Expires)),
		Entry: entry,
	}
}

type timer struct {
	timer *time.Timer
	Entry *cacheEntryExpiry
}

func (t *timer) Wait() <-chan time.Time {
	if t.timer == nil {
		return nil
	}
	return t.timer.C
}

func (t *timer) Stop() {
	if t.timer != nil {
		t.timer.Stop()
	}
}
