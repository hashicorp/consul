// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package queue

import (
	"context"
	"sync"
	"time"
)

// much of this is a re-implementation of
// https://github.com/kubernetes/client-go/blob/release-1.25/util/workqueue/queue.go

// ItemType is the type constraint for items in the WorkQueue.
type ItemType interface {
	// Key returns a string that will be used to de-duplicate items in the queue.
	Key() string
}

// WorkQueue is an interface for a work queue with semantics to help with
// retries and rate limiting.
type WorkQueue[T ItemType] interface {
	// Get retrieves the next Request in the queue, blocking until a Request is
	// available, if shutdown is true, then the queue is shutting down and should
	// no longer be used by the caller.
	Get() (item T, shutdown bool)
	// Add immediately adds a Request to the work queue.
	Add(item T)
	// AddAfter adds a Request to the work queue after a given amount of time.
	AddAfter(item T, duration time.Duration)
	// AddRateLimited adds a Request to the work queue after the amount of time
	// specified by applying the queue's rate limiter.
	AddRateLimited(item T)
	// Forget signals the queue to reset the rate-limiting for the given Request.
	Forget(item T)
	// Done tells the work queue that the Request has been successfully processed
	// and can be deleted from the queue.
	Done(item T)
}

// queue implements a rate-limited work queue
type queue[T ItemType] struct {
	// queue holds an ordered list of Requests needing to be processed
	queue []T

	// dirty holds the working set of all Requests, whether they are being
	// processed or not
	dirty map[string]struct{}
	// processing holds the set of current requests being processed
	processing map[string]struct{}

	// deferred is an internal priority queue that tracks deferred
	// Requests
	deferred DeferQueue[T]
	// ratelimiter is the internal rate-limiter for the queue
	ratelimiter Limiter[T]

	// cond synchronizes queue access and handles signalling for when
	// data is available in the queue
	cond *sync.Cond

	// ctx is the top-level context that, when canceled, shuts down the queue
	ctx context.Context
}

// RunWorkQueue returns a started WorkQueue that has per-item exponential backoff rate-limiting.
// When the passed in context is canceled, the queue shuts down.
func RunWorkQueue[T ItemType](ctx context.Context, baseBackoff, maxBackoff time.Duration) WorkQueue[T] {
	q := &queue[T]{
		ratelimiter: NewRateLimiter[T](baseBackoff, maxBackoff),
		dirty:       make(map[string]struct{}),
		processing:  make(map[string]struct{}),
		cond:        sync.NewCond(&sync.Mutex{}),
		deferred:    NewDeferQueue[T](500 * time.Millisecond),
		ctx:         ctx,
	}
	go q.start()

	return q
}

// start begins the asynchronous processing loop for the deferral queue
func (q *queue[T]) start() {
	go q.deferred.Process(q.ctx, func(item T) {
		q.Add(item)
	})

	<-q.ctx.Done()
	q.cond.Broadcast()
}

// shuttingDown returns whether the queue is in the process of shutting down
func (q *queue[T]) shuttingDown() bool {
	select {
	case <-q.ctx.Done():
		return true
	default:
		return false
	}
}

// Get returns the next Request to be processed by the caller, blocking until
// an item is available in the queue. If the returned shutdown parameter is true,
// then the caller should stop using the queue. Any Requests returned by a call
// to Get must be explicitly marked as processed via the Done method.
func (q *queue[T]) Get() (item T, shutdown bool) {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()
	for len(q.queue) == 0 && !q.shuttingDown() {
		q.cond.Wait()
	}
	if len(q.queue) == 0 {
		// We must be shutting down.
		var zero T
		return zero, true
	}

	item, q.queue = q.queue[0], q.queue[1:]

	q.processing[item.Key()] = struct{}{}
	delete(q.dirty, item.Key())

	return item, false
}

// Add puts the given Request in the queue. If the Request is already in
// the queue or the queue is stopping, then this is a no-op.
func (q *queue[T]) Add(item T) {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()
	if q.shuttingDown() {
		return
	}
	if _, ok := q.dirty[item.Key()]; ok {
		return
	}

	q.dirty[item.Key()] = struct{}{}
	if _, ok := q.processing[item.Key()]; ok {
		return
	}

	q.queue = append(q.queue, item)
	q.cond.Signal()
}

// AddAfter adds a Request to the work queue after a given amount of time.
func (q *queue[T]) AddAfter(item T, duration time.Duration) {
	// don't add if we're already shutting down
	if q.shuttingDown() {
		return
	}

	// immediately add if there is no delay
	if duration <= 0 {
		q.Add(item)
		return
	}

	q.deferred.Defer(q.ctx, item, time.Now().Add(duration))
}

// AddRateLimited adds the given Request to the queue after applying the
// rate limiter to determine when the Request should next be processed.
func (q *queue[T]) AddRateLimited(item T) {
	q.AddAfter(item, q.ratelimiter.NextRetry(item))
}

// Forget signals the queue to reset the rate-limiting for the given Request.
func (q *queue[T]) Forget(item T) {
	q.ratelimiter.Forget(item)
}

// Done removes the item from the queue, if it has been marked dirty
// again while being processed, it is re-added to the queue.
func (q *queue[T]) Done(item T) {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()

	delete(q.processing, item.Key())
	if _, ok := q.dirty[item.Key()]; ok {
		q.queue = append(q.queue, item)
		q.cond.Signal()
	}
}
