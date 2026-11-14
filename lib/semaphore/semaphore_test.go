// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package semaphore

// Based on https://github.com/golang/sync/blob/master/semaphore/semaphore_test.go

import (
	"context"
	"math/rand"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const maxSleep = 1 * time.Millisecond

func HammerDynamic(sem *Dynamic, loops int) {
	for i := 0; i < loops; i++ {
		sem.Acquire(context.Background())
		time.Sleep(time.Duration(rand.Int63n(int64(maxSleep/time.Nanosecond))) * time.Nanosecond)
		sem.Release()
	}
}

// TestDynamic hammers the semaphore from all available cores to ensure we don't
// hit a panic or race detector notice something wonky.
func TestDynamic(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	n := runtime.GOMAXPROCS(0)
	loops := 10000 / n
	sem := NewDynamic(int64(n))
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			HammerDynamic(sem, loops)
		}()
	}
	wg.Wait()
}

func TestDynamicPanic(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Fatal("release of an unacquired dynamic semaphore did not panic")
		}
	}()
	w := NewDynamic(1)
	w.Release()
}

// TestDynamicShrinkWithWaiters verifies that when the semaphore is shrunk below
// the number of currently acquired slots while other goroutines are blocked
// waiting, releasing a slot does not hand it to a waiter. Doing so would let the
// waiter acquire while the outstanding count is still above the reduced size,
// violating the SetSize contract.
func TestDynamicShrinkWithWaiters(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sem := NewDynamic(2)

	// Fill both slots [free: 0].
	require.NoError(t, sem.Acquire(ctx))
	require.NoError(t, sem.Acquire(ctx))

	// Start a goroutine that blocks waiting for a slot and records when it
	// manages to acquire one.
	acquired := make(chan struct{})
	go func() {
		_ = sem.Acquire(context.Background())
		close(acquired)
	}()

	// Give the goroutine time to block and register itself as a waiter.
	select {
	case <-acquired:
		t.Fatal("acquire unexpectedly succeeded while the semaphore was full")
	case <-time.After(100 * time.Millisecond):
	}

	// Shrink the semaphore below the number of acquired slots (cur=2, size=1).
	require.NoError(t, sem.SetSize(1))

	// Releasing one slot still leaves us at capacity (outstanding 1, size 1), so
	// the waiter must remain blocked rather than being handed the freed slot.
	sem.Release()
	select {
	case <-acquired:
		t.Fatal("waiter acquired a slot while the semaphore was over its reduced capacity")
	case <-time.After(100 * time.Millisecond):
	}

	// Releasing the final outstanding slot brings us under capacity, at which
	// point the waiter is allowed to proceed.
	sem.Release()
	select {
	case <-acquired:
	case <-time.After(time.Second):
		t.Fatal("waiter was not granted a slot after capacity freed up")
	}
}

func checkAcquire(t *testing.T, sem *Dynamic, wantAcquire bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	err := sem.Acquire(ctx)
	if wantAcquire {
		require.NoErrorf(t, err, "failed to acquire when we should have")
	} else {
		require.Error(t, err, "failed to block when should be full")
	}
}

func TestDynamicAcquire(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sem := NewDynamic(2)

	// Consume one slot [free: 1]
	sem.Acquire(ctx)
	// Should be able to consume another [free: 0]
	checkAcquire(t, sem, true)
	// Should fail to consume another [free: 0]
	checkAcquire(t, sem, false)

	// Release 2
	sem.Release()
	sem.Release()

	// Should be able to consume another [free: 1]
	checkAcquire(t, sem, true)
	// Should be able to consume another [free: 0]
	checkAcquire(t, sem, true)
	// Should fail to consume another [free: 0]
	checkAcquire(t, sem, false)

	// Now expand the semaphore and we should be able to acquire again [free: 2]
	sem.SetSize(4)

	// Should be able to consume another [free: 1]
	checkAcquire(t, sem, true)
	// Should be able to consume another [free: 0]
	checkAcquire(t, sem, true)
	// Should fail to consume another [free: 0]
	checkAcquire(t, sem, false)

	// Shrinking it should work [free: 0]
	sem.SetSize(3)

	// Should fail to consume another [free: 0]
	checkAcquire(t, sem, false)

	// Release one [free: 0] (3 slots used are release, size only 3)
	sem.Release()

	// Should fail to consume another [free: 0]
	checkAcquire(t, sem, false)

	sem.Release()

	// Should be able to consume another [free: 1]
	checkAcquire(t, sem, true)
}
