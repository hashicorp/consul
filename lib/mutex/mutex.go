// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

/*
Package mutex implements the sync.Locker interface using x/sync/semaphore. It
may be used as a replacement for sync.Mutex when one or more goroutines need to
allow their calls to Lock to be cancelled by context cancellation.
*/
package mutex

import (
	"context"

	"golang.org/x/sync/semaphore"
)

type Mutex semaphore.Weighted

// New returns a Mutex that is ready for use.
func New() *Mutex {
	return (*Mutex)(semaphore.NewWeighted(1))
}

func (m *Mutex) Lock() {
	_ = (*semaphore.Weighted)(m).Acquire(context.Background(), 1)
}

func (m *Mutex) Unlock() {
	(*semaphore.Weighted)(m).Release(1)
}

// TryLock acquires the mutex, blocking until resources are available or ctx is
// done. On success, returns nil. On failure, returns ctx.Err() and leaves the
// semaphore unchanged.
//
// If ctx is already done, Acquire may still succeed without blocking.
func (m *Mutex) TryLock(ctx context.Context) error {
	return (*semaphore.Weighted)(m).Acquire(ctx, 1)
}
