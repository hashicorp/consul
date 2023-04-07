package controller

import (
	"context"
	"sync"
	"time"
)

// much of this is a re-implementation of
// https://github.com/kubernetes/client-go/blob/release-1.25/util/workqueue/queue.go

// WorkQueue is an interface for a work queue with semantics to help with
// retries and rate limiting.
type WorkQueue interface {
	// Get retrieves the next Request in the queue, blocking until a Request is
	// available, if shutdown is true, then the queue is shutting down and should
	// no longer be used by the caller.
	Get() (item Request, shutdown bool)
	// Add immediately adds a Request to the work queue.
	Add(item Request)
	// AddAfter adds a Request to the work queue after a given amount of time.
	AddAfter(item Request, duration time.Duration)
	// AddRateLimited adds a Request to the work queue after the amount of time
	// specified by applying the queue's rate limiter.
	AddRateLimited(item Request)
	// Forget signals the queue to reset the rate-limiting for the given Request.
	Forget(item Request)
	// Done tells the work queue that the Request has been successfully processed
	// and can be deleted from the queue.
	Done(item Request)
}

// queue implements a rate-limited work queue
type queue struct {
	// queue holds an ordered list of Requests needing to be processed
	queue []Request

	// dirty holds the working set of all Requests, whether they are being
	// processed or not
	dirty map[Request]struct{}
	// processing holds the set of current requests being processed
	processing map[Request]struct{}

	// deferred is an internal priority queue that tracks deferred
	// Requests
	deferred DeferQueue
	// ratelimiter is the internal rate-limiter for the queue
	ratelimiter Limiter

	// cond synchronizes queue access and handles signalling for when
	// data is available in the queue
	cond *sync.Cond

	// ctx is the top-level context that, when canceled, shuts down the queue
	ctx context.Context
}

// RunWorkQueue returns a started WorkQueue that has per-Request exponential backoff rate-limiting.
// When the passed in context is canceled, the queue shuts down.
func RunWorkQueue(ctx context.Context, baseBackoff, maxBackoff time.Duration) WorkQueue {
	q := &queue{
		ratelimiter: NewRateLimiter(baseBackoff, maxBackoff),
		dirty:       make(map[Request]struct{}),
		processing:  make(map[Request]struct{}),
		cond:        sync.NewCond(&sync.Mutex{}),
		deferred:    NewDeferQueue(500 * time.Millisecond),
		ctx:         ctx,
	}
	go q.start()

	return q
}

// start begins the asynchronous processing loop for the deferral queue
func (q *queue) start() {
	go q.deferred.Process(q.ctx, func(item Request) {
		q.Add(item)
	})

	<-q.ctx.Done()
	q.cond.Broadcast()
}

// shuttingDown returns whether the queue is in the process of shutting down
func (q *queue) shuttingDown() bool {
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
func (q *queue) Get() (item Request, shutdown bool) {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()
	for len(q.queue) == 0 && !q.shuttingDown() {
		q.cond.Wait()
	}
	if len(q.queue) == 0 {
		// We must be shutting down.
		return Request{}, true
	}

	item, q.queue = q.queue[0], q.queue[1:]

	q.processing[item] = struct{}{}
	delete(q.dirty, item)

	return item, false
}

// Add puts the given Request in the queue. If the Request is already in
// the queue or the queue is stopping, then this is a no-op.
func (q *queue) Add(item Request) {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()
	if q.shuttingDown() {
		return
	}
	if _, ok := q.dirty[item]; ok {
		return
	}

	q.dirty[item] = struct{}{}
	if _, ok := q.processing[item]; ok {
		return
	}

	q.queue = append(q.queue, item)
	q.cond.Signal()
}

// AddAfter adds a Request to the work queue after a given amount of time.
func (q *queue) AddAfter(item Request, duration time.Duration) {
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
func (q *queue) AddRateLimited(item Request) {
	q.AddAfter(item, q.ratelimiter.NextRetry(item))
}

// Forget signals the queue to reset the rate-limiting for the given Request.
func (q *queue) Forget(item Request) {
	q.ratelimiter.Forget(item)
}

// Done removes the item from the queue, if it has been marked dirty
// again while being processed, it is re-added to the queue.
func (q *queue) Done(item Request) {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()

	delete(q.processing, item)
	if _, ok := q.dirty[item]; ok {
		q.queue = append(q.queue, item)
		q.cond.Signal()
	}
}
