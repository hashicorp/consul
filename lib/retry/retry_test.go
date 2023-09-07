// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package retry

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestJitter(t *testing.T) {
	repeat(t, "0 percent", func(t *testing.T) {
		jitter := NewJitter(0)
		for i := 0; i < 10; i++ {
			baseTime := time.Duration(i) * time.Second
			require.Equal(t, baseTime, jitter(baseTime))
		}
	})

	repeat(t, "10 percent", func(t *testing.T) {
		jitter := NewJitter(10)
		baseTime := 5000 * time.Millisecond
		maxTime := 5500 * time.Millisecond
		newTime := jitter(baseTime)
		require.True(t, newTime > baseTime)
		require.True(t, newTime <= maxTime)
	})

	repeat(t, "100 percent", func(t *testing.T) {
		jitter := NewJitter(100)
		baseTime := 1234 * time.Millisecond
		maxTime := 2468 * time.Millisecond
		newTime := jitter(baseTime)
		require.True(t, newTime > baseTime)
		require.True(t, newTime <= maxTime)
	})

	repeat(t, "overflow", func(t *testing.T) {
		jitter := NewJitter(100)
		baseTime := time.Duration(math.MaxInt64) - 2*time.Hour
		newTime := jitter(baseTime)
		require.Equal(t, baseTime, newTime)
	})
}

func repeat(t *testing.T, name string, fn func(t *testing.T)) {
	t.Run(name, func(t *testing.T) {
		for i := 0; i < 1000; i++ {
			fn(t)
		}
	})
}

func TestWaiter_Delay(t *testing.T) {
	t.Run("zero value", func(t *testing.T) {
		w := &Waiter{}
		for i, expected := range []time.Duration{0, 1, 2, 4, 8, 16, 32, 64, 128} {
			w.failures = uint(i)
			require.Equal(t, expected*time.Second, w.delay(), "failure count: %d", i)
		}
	})

	t.Run("with minimum wait", func(t *testing.T) {
		w := &Waiter{MinWait: 5 * time.Second}
		for i, expected := range []time.Duration{5, 5, 5, 5, 8, 16, 32, 64, 128} {
			w.failures = uint(i)
			require.Equal(t, expected*time.Second, w.delay(), "failure count: %d", i)
		}
	})

	t.Run("with maximum wait", func(t *testing.T) {
		w := &Waiter{MaxWait: 20 * time.Second}
		for i, expected := range []time.Duration{0, 1, 2, 4, 8, 16, 20, 20, 20} {
			w.failures = uint(i)
			require.Equal(t, expected*time.Second, w.delay(), "failure count: %d", i)
		}
	})

	t.Run("with minimum failures", func(t *testing.T) {
		w := &Waiter{MinFailures: 4}
		for i, expected := range []time.Duration{0, 0, 0, 0, 0, 1, 2, 4, 8, 16} {
			w.failures = uint(i)
			require.Equal(t, expected*time.Second, w.delay(), "failure count: %d", i)
		}
	})

	t.Run("with factor", func(t *testing.T) {
		w := &Waiter{Factor: time.Millisecond}
		for i, expected := range []time.Duration{0, 1, 2, 4, 8, 16, 32, 64, 128} {
			w.failures = uint(i)
			require.Equal(t, expected*time.Millisecond, w.delay(), "failure count: %d", i)
		}
	})

	t.Run("with all settings", func(t *testing.T) {
		w := &Waiter{
			MinFailures: 2,
			MinWait:     4 * time.Millisecond,
			MaxWait:     20 * time.Millisecond,
			Factor:      time.Millisecond,
		}
		for i, expected := range []time.Duration{4, 4, 4, 4, 4, 4, 8, 16, 20, 20, 20} {
			w.failures = uint(i)
			require.Equal(t, expected*time.Millisecond, w.delay(), "failure count: %d", i)
		}
	})

	t.Run("jitter can exceed MaxWait", func(t *testing.T) {
		w := &Waiter{
			MaxWait: 20 * time.Second,
			Jitter:  NewJitter(300),

			failures: 30,
		}

		delay := w.delay()
		require.True(t, delay > 20*time.Second, "expected delay %v to be greater than MaxWait %v", delay, w.MaxWait)
	})
}

func TestWaiter_Wait(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	ctx := context.Background()

	t.Run("first failure", func(t *testing.T) {
		w := &Waiter{MinWait: time.Millisecond, Factor: 1}
		elapsed, err := runWait(ctx, w)
		require.NoError(t, err)
		assertApproximateDuration(t, elapsed, time.Millisecond)
		require.Equal(t, w.failures, uint(1))
	})

	t.Run("max failures", func(t *testing.T) {
		w := &Waiter{
			MaxWait:  100 * time.Millisecond,
			failures: 200,
		}
		elapsed, err := runWait(ctx, w)
		require.NoError(t, err)
		assertApproximateDuration(t, elapsed, 100*time.Millisecond)
		require.Equal(t, w.failures, uint(201))
	})

	t.Run("context deadline", func(t *testing.T) {
		w := &Waiter{failures: 200, MinWait: time.Second}
		ctx, cancel := context.WithTimeout(ctx, 5*time.Millisecond)
		t.Cleanup(cancel)

		elapsed, err := runWait(ctx, w)
		require.Equal(t, err, context.DeadlineExceeded)
		assertApproximateDuration(t, elapsed, 5*time.Millisecond)
		require.Equal(t, w.failures, uint(201))
	})
}

func TestWaiter_RetryLoop(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	// Change the default factor so that we retry faster.
	w := &Waiter{Factor: 1 * time.Millisecond}

	t.Run("exits if operation is successful after a few reties", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		t.Cleanup(cancel)
		numRetries := 0
		err := w.RetryLoop(ctx, func() error {
			if numRetries < 2 {
				numRetries++
				return fmt.Errorf("operation not successful")
			}
			return nil
		})
		require.NoError(t, err)
	})

	t.Run("errors if operation is never successful", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		t.Cleanup(cancel)
		err := w.RetryLoop(ctx, func() error {
			return fmt.Errorf("operation not successful")
		})
		require.NotNil(t, err)
		require.EqualError(t, err, "could not retry operation: operation not successful")
	})
}

func runWait(ctx context.Context, w *Waiter) (time.Duration, error) {
	before := time.Now()
	err := w.Wait(ctx)
	return time.Since(before), err
}

func assertApproximateDuration(t *testing.T, actual time.Duration, expected time.Duration) {
	t.Helper()
	delta := 20 * time.Millisecond
	min, max := expected-delta, expected+delta
	if min < 0 {
		min = 0
	}
	if actual < min || actual > max {
		t.Fatalf("expected %v to be between %v and %v", actual, min, max)
	}
}
