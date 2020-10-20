package ttlcache

import (
	"container/heap"
	"time"
)

// Entry in the ExpiryHeap, tracks the index and expiry time of an item in a
// ttl cache.
type Entry struct {
	// TODO: can Key be unexported?
	Key       string
	expiry    time.Time
	heapIndex int
}

func (c *Entry) Index() int {
	if c == nil {
		return -1
	}
	return c.heapIndex
}

// ExpiryHeap is a container/heap.Interface implementation that expires entries
// in the cache when their expiration time is reached.
//
// All operations on the heap and read/write of the heap contents require
// the proper entriesLock to be held on Cache.
type ExpiryHeap struct {
	entries []*Entry

	// NotifyCh is sent a value whenever the 0 index value of the heap
	// changes. This can be used to detect when the earliest value
	// changes.
	NotifyCh chan struct{}
}

// NewExpiryHeap creates and returns a new ExpiryHeap.
func NewExpiryHeap() *ExpiryHeap {
	h := &ExpiryHeap{NotifyCh: make(chan struct{}, 1)}
	heap.Init((*entryHeap)(h))
	return h
}

// Add an entry to the heap.
//
// Must be synchronized by the caller.
func (h *ExpiryHeap) Add(key string, expiry time.Duration) *Entry {
	entry := &Entry{
		Key:    key,
		expiry: time.Now().Add(expiry),
		// Set the initial heap index to the last index. If the entry is swapped it
		// will have the correct index set, and if it remains at the end the last
		// index will be correct.
		heapIndex: len(h.entries),
	}
	heap.Push((*entryHeap)(h), entry)
	if entry.heapIndex == 0 {
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
	entry.expiry = time.Now().Add(expiry)
	heap.Fix((*entryHeap)(h), idx)

	// If the previous index and current index are both zero then Fix did not
	// swap the entry, and notify must be called here.
	if idx == 0 || entry.heapIndex == 0 {
		h.notify()
	}
}

// Remove the entry at idx from the heap.
//
// Must be synchronized by the caller.
func (h *ExpiryHeap) Remove(idx int) {
	entry := h.entries[idx]
	heap.Remove((*entryHeap)(h), idx)

	// A goroutine which is fetching a new value will have a reference to this
	// entry. When it re-acquires the lock it needs to be informed that
	// the entry was expired while it was fetching. Setting heapIndex to -1
	// indicates that the entry is no longer in the heap, and must be re-added.
	entry.heapIndex = -1

	if idx == 0 {
		h.notify()
	}
}

type entryHeap ExpiryHeap

func (h *entryHeap) Len() int { return len(h.entries) }

func (h *entryHeap) Swap(i, j int) {
	h.entries[i], h.entries[j] = h.entries[j], h.entries[i]
	h.entries[i].heapIndex = i
	h.entries[j].heapIndex = j
}

func (h *entryHeap) Less(i, j int) bool {
	// The usage of Before here is important (despite being obvious):
	// this function uses the monotonic time that should be available
	// on the time.Time value so the heap is immune to wall clock changes.
	return h.entries[i].expiry.Before(h.entries[j].expiry)
}

// heap.Interface, this isn't expected to be called directly.
func (h *entryHeap) Push(x interface{}) {
	h.entries = append(h.entries, x.(*Entry))
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

// Next returns a Timer that waits until the first entry in the heap expires.
//
// Must be synchronized by the caller.
func (h *ExpiryHeap) Next() Timer {
	if len(h.entries) == 0 {
		return Timer{}
	}
	entry := h.entries[0]
	return Timer{
		timer: time.NewTimer(time.Until(entry.expiry)),
		Entry: entry,
	}
}

type Timer struct {
	timer *time.Timer
	Entry *Entry
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
