// Copyright (c) HashiCorp, Inc.
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
