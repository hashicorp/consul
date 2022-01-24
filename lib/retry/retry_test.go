package retry

import (
	"context"
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
