// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package semaphore implements a simple semaphore that is based on
// golang.org/x/sync/semaphore but doesn't support weights. It's advantage over
// a simple buffered chan is that the capacity of the semaphore (i.e. the number
// of slots available) can be changed dynamically at runtime without waiting for
// all existing work to stop. This makes it easier to implement e.g. concurrency
// limits on certain operations that can be reconfigured at runtime.
package semaphore

import (
	"container/list"
	"context"
	"sync"
)

// Dynamic implements a semaphore whose capacity can be changed dynamically at
// run time.
type Dynamic struct {
	size    int64
	cur     int64
	waiters list.List
	mu      sync.Mutex
}

// NewDynamic returns a dynamic semaphore with the given initial capacity. Note
// that this is for convenience and to match golang.org/x/sync/semaphore however
// it's possible to use a zero-value semaphore provided SetSize is called before
// use.
func NewDynamic(n int64) *Dynamic {
	return &Dynamic{
		size: n,
	}
}

// SetSize dynamically updates the number of available slots. If there are more
// than n slots currently acquired, no further acquires will succeed until
// sufficient have been released to take the total outstanding below n again.
func (s *Dynamic) SetSize(n int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.size = n
	return nil
}

// Acquire attempts to acquire one "slot" in the semaphore, blocking only until
// ctx is Done. On success, returns nil. On failure, returns ctx.Err() and leaves
// the semaphore unchanged.
//
// If ctx is already done, Acquire may still succeed without blocking.
func (s *Dynamic) Acquire(ctx context.Context) error {
	s.mu.Lock()
	if s.cur < s.size {
		s.cur++
		s.mu.Unlock()
		return nil
	}

	// Need to wait, add to waiter list
	ready := make(chan struct{})
	elem := s.waiters.PushBack(ready)
	s.mu.Unlock()

	select {
	case <-ctx.Done():
		err := ctx.Err()
		s.mu.Lock()
		select {
		case <-ready:
			// Acquired the semaphore after we were canceled.  Rather than trying to
			// fix up the queue, just pretend we didn't notice the cancellation.
			err = nil
		default:
			s.waiters.Remove(elem)
		}
		s.mu.Unlock()
		return err

	case <-ready:
		return nil
	}
}

// Release releases the semaphore. It will panic if release is called on an
// empty semphore.
func (s *Dynamic) Release() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cur < 1 {
		panic("semaphore: bad release")
	}

	next := s.waiters.Front()

	// If there are no waiters, just decrement and we're done
	if next == nil {
		s.cur--
		return
	}

	// Need to yield our slot to the next waiter.
	// Remove them from the list
	s.waiters.Remove(next)
	// And trigger it's chan before we release the lock
	close(next.Value.(chan struct{}))
	// Note we _don't_ decrement inflight since the slot was yielded directly.
}
