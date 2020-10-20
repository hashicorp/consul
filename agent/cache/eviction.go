package cache

import (
	"container/heap"
	"time"
)

// CacheEntryExpiry contains the expiration time for a cache entry.
type CacheEntryExpiry struct {
	Key       string    // Key in the cache map
	Expires   time.Time // Time when entry expires (monotonic clock)
	HeapIndex int       // Index in the heap
}

// ExpiryHeap is a container/heap.Interface implementation that expires entries
// in the cache when their expiration time is reached.
//
// All operations on the heap and read/write of the heap contents require
// the proper entriesLock to be held on Cache.
type ExpiryHeap struct {
	entries []*CacheEntryExpiry

	// NotifyCh is sent a value whenever the 0 index value of the heap
	// changes. This can be used to detect when the earliest value
	// changes.
	NotifyCh chan struct{}
}

// Initialize the heap. The buffer of 1 is really important because
// its possible for the expiry loop to trigger the heap to update
// itself and it'd block forever otherwise.
func NewExpiryHeap() *ExpiryHeap {
	h := &ExpiryHeap{NotifyCh: make(chan struct{}, 1)}
	heap.Init((*entryHeap)(h))
	return h
}

// Add an entry to the heap.
//
// Must be synchronized by the caller.
func (h *ExpiryHeap) Add(key string, expiry time.Duration) *CacheEntryExpiry {
	entry := &CacheEntryExpiry{
		Key:     key,
		Expires: time.Now().Add(expiry),
		// Set the initial heap index to the last index. If the entry is swapped it
		// will have the correct index set, and if it remains at the end the last
		// index will be correct.
		HeapIndex: len(h.entries),
	}
	heap.Push((*entryHeap)(h), entry)
	if entry.HeapIndex == 0 {
		h.notify()
	}
	return entry
}

// Update the entry that is currently at idx with the new expiry time. The heap
// will be rebalanced after the entry is updated.
//
// Must be synchronized by the caller.
func (h *ExpiryHeap) Update(idx int, expiry time.Duration) {
	entry := h.entries[idx]
	entry.Expires = time.Now().Add(expiry)
	heap.Fix((*entryHeap)(h), idx)

	// If the previous index and current index are both zero then Fix did not
	// swap the entry, and notify must be called here.
	if idx == 0 || entry.HeapIndex == 0 {
		h.notify()
	}
}

// Must be synchronized by the caller.
func (h *ExpiryHeap) Remove(idx int) {
	entry := h.entries[idx]
	heap.Remove((*entryHeap)(h), idx)

	// A goroutine which is fetching a new value will have a reference to this
	// entry. When it re-acquires the lock it needs to be informed that
	// the entry was expired while it was fetching. Setting HeapIndex to -1
	// indicates that the entry is no longer in the heap, and must be re-added.
	entry.HeapIndex = -1

	if idx == 0 {
		h.notify()
	}
}

type entryHeap ExpiryHeap

func (h *entryHeap) Len() int { return len(h.entries) }

func (h *entryHeap) Swap(i, j int) {
	h.entries[i], h.entries[j] = h.entries[j], h.entries[i]
	h.entries[i].HeapIndex = i
	h.entries[j].HeapIndex = j
}

func (h *entryHeap) Less(i, j int) bool {
	// The usage of Before here is important (despite being obvious):
	// this function uses the monotonic time that should be available
	// on the time.Time value so the heap is immune to wall clock changes.
	return h.entries[i].Expires.Before(h.entries[j].Expires)
}

// heap.Interface, this isn't expected to be called directly.
func (h *entryHeap) Push(x interface{}) {
	h.entries = append(h.entries, x.(*CacheEntryExpiry))
}

// heap.Interface, this isn't expected to be called directly.
func (h *entryHeap) Pop() interface{} {
	n := len(h.entries)
	entries := h.entries
	last := entries[n-1]
	h.entries = entries[0 : n-1]
	return last
}

// notify the timer that the head value has changed, so the expiry time has
// also likely changed.
func (h *ExpiryHeap) notify() {
	// Send to channel without blocking. Skips sending if there is already
	// an item in the buffered channel.
	select {
	case h.NotifyCh <- struct{}{}:
	default:
	}
}

// Must be synchronized by the caller.
func (h *ExpiryHeap) Next() Timer {
	if len(h.entries) == 0 {
		return Timer{}
	}
	entry := h.entries[0]
	return Timer{
		timer: time.NewTimer(time.Until(entry.Expires)),
		Entry: entry,
	}
}

type Timer struct {
	timer *time.Timer
	Entry *CacheEntryExpiry
}

func (t *Timer) Wait() <-chan time.Time {
	if t.timer == nil {
		return nil
	}
	return t.timer.C
}

func (t *Timer) Stop() {
	if t.timer != nil {
		t.timer.Stop()
	}
}
