// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package queue

import (
	"container/heap"
	"context"
	"time"
)

// much of this is a re-implementation of
// https://github.com/kubernetes/client-go/blob/release-1.25/util/workqueue/delaying_queue.go

// DeferQueue is a generic priority queue implementation that
// allows for deferring and later processing Requests.
type DeferQueue[T ItemType] interface {
	// Defer defers processing a Request until a given time. When
	// the timeout is hit, the request will be processed by the
	// callback given in the Process loop. If the given context
	// is canceled, the item is not deferred.
	Defer(ctx context.Context, item T, until time.Time)
	// Process processes all items in the defer queue with the
	// given callback, blocking until the given context is canceled.
	// Callers should only ever call Process once, likely in a
	// long-lived goroutine.
	Process(ctx context.Context, callback func(item T))
}

// deferredRequest is a wrapped Request with information about
// when a retry should be attempted
type deferredRequest[T ItemType] struct {
	enqueueAt time.Time
	item      T
	// index holds the index for the given heap entry so that if
	// the entry is updated the heap can be re-sorted
	index int
}

// deferQueue is a priority queue for deferring Requests for
// future processing
type deferQueue[T ItemType] struct {
	heap    *deferHeap[T]
	entries map[string]*deferredRequest[T]

	addChannel     chan *deferredRequest[T]
	heartbeat      *time.Ticker
	nextReadyTimer *time.Timer
}

// NewDeferQueue returns a priority queue for deferred Requests.
func NewDeferQueue[T ItemType](tick time.Duration) DeferQueue[T] {
	dHeap := &deferHeap[T]{}
	heap.Init(dHeap)

	return &deferQueue[T]{
		heap:       dHeap,
		entries:    make(map[string]*deferredRequest[T]),
		addChannel: make(chan *deferredRequest[T]),
		heartbeat:  time.NewTicker(tick),
	}
}

// Defer defers the given Request until the given time in the future. If the
// passed in context is canceled before the Request is deferred, then this
// immediately returns.
func (q *deferQueue[T]) Defer(ctx context.Context, item T, until time.Time) {
	entry := &deferredRequest[T]{
		enqueueAt: until,
		item:      item,
	}

	select {
	case <-ctx.Done():
	case q.addChannel <- entry:
	}
}

// deferEntry adds a deferred request to the priority queue
func (q *deferQueue[T]) deferEntry(entry *deferredRequest[T]) {
	existing, exists := q.entries[entry.item.Key()]
	if exists {
		// insert or update the item deferral time
		if existing.enqueueAt.After(entry.enqueueAt) {
			existing.enqueueAt = entry.enqueueAt
			heap.Fix(q.heap, existing.index)
		}

		return
	}

	heap.Push(q.heap, entry)
	q.entries[entry.item.Key()] = entry
}

// readyRequest returns a pointer to the next ready Request or
// nil if no Requests are ready to be processed
func (q *deferQueue[T]) readyRequest() *T {
	if q.heap.Len() == 0 {
		return nil
	}

	now := time.Now()

	entry := q.heap.Peek().(*deferredRequest[T])
	if entry.enqueueAt.After(now) {
		return nil
	}

	entry = heap.Pop(q.heap).(*deferredRequest[T])
	delete(q.entries, entry.item.Key())
	return &entry.item
}

// signalReady returns a timer signal to the next Request
// that will be ready on the queue
func (q *deferQueue[T]) signalReady() <-chan time.Time {
	if q.heap.Len() == 0 {
		return make(<-chan time.Time)
	}

	if q.nextReadyTimer != nil {
		q.nextReadyTimer.Stop()
	}
	now := time.Now()
	entry := q.heap.Peek().(*deferredRequest[T])
	q.nextReadyTimer = time.NewTimer(entry.enqueueAt.Sub(now))
	return q.nextReadyTimer.C
}

// Process processes all items in the defer queue with the
// given callback, blocking until the given context is canceled.
// Callers should only ever call Process once, likely in a
// long-lived goroutine.
func (q *deferQueue[T]) Process(ctx context.Context, callback func(item T)) {
	for {
		ready := q.readyRequest()
		if ready != nil {
			callback(*ready)
		}

		signalReady := q.signalReady()

		select {
		case <-ctx.Done():
			if q.nextReadyTimer != nil {
				q.nextReadyTimer.Stop()
			}
			q.heartbeat.Stop()
			return

		case <-q.heartbeat.C:
			// continue the loop, which process ready items

		case <-signalReady:
			// continue the loop, which process ready items

		case entry := <-q.addChannel:
			enqueueOrProcess := func(entry *deferredRequest[T]) {
				now := time.Now()
				if entry.enqueueAt.After(now) {
					q.deferEntry(entry)
				} else {
					// fast-path, process immediately if we don't need to defer
					callback(entry.item)
				}
			}

			enqueueOrProcess(entry)

			// drain the add channel before we do anything else
			drained := false
			for !drained {
				select {
				case entry := <-q.addChannel:
					enqueueOrProcess(entry)
				default:
					drained = true
				}
			}
		}
	}
}

// deferHeap implements heap.Interface
type deferHeap[T ItemType] []*deferredRequest[T]

// Len returns the length of the heap.
func (h deferHeap[T]) Len() int {
	return len(h)
}

// Less compares heap items for purposes of sorting.
func (h deferHeap[T]) Less(i, j int) bool {
	return h[i].enqueueAt.Before(h[j].enqueueAt)
}

// Swap swaps two entries in the heap.
func (h deferHeap[T]) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

// Push pushes an entry onto the heap.
func (h *deferHeap[T]) Push(x interface{}) {
	n := len(*h)
	item := x.(*deferredRequest[T])
	item.index = n
	*h = append(*h, item)
}

// Pop pops an entry off the heap.
func (h *deferHeap[T]) Pop() interface{} {
	n := len(*h)
	item := (*h)[n-1]
	item.index = -1
	*h = (*h)[0:(n - 1)]
	return item
}

// Peek returns the next item on the heap.
func (h deferHeap[T]) Peek() interface{} {
	return h[0]
}
