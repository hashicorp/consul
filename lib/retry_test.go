package lib

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestJitterRandomStagger(t *testing.T) {
	t.Parallel()

	t.Run("0 percent", func(t *testing.T) {
		t.Parallel()
		jitter := NewJitterRandomStagger(0)
		for i := 0; i < 10; i++ {
			baseTime := time.Duration(i) * time.Second
			require.Equal(t, baseTime, jitter.AddJitter(baseTime))
		}
	})

	t.Run("10 percent", func(t *testing.T) {
		t.Parallel()
		jitter := NewJitterRandomStagger(10)
		for i := 0; i < 10; i++ {
			baseTime := 5000 * time.Millisecond
			maxTime := 5500 * time.Millisecond
			newTime := jitter.AddJitter(baseTime)
			require.True(t, newTime > baseTime)
			require.True(t, newTime <= maxTime)
		}
	})

	t.Run("100 percent", func(t *testing.T) {
		t.Parallel()
		jitter := NewJitterRandomStagger(100)
		for i := 0; i < 10; i++ {
			baseTime := 1234 * time.Millisecond
			maxTime := 2468 * time.Millisecond
			newTime := jitter.AddJitter(baseTime)
			require.True(t, newTime > baseTime)
			require.True(t, newTime <= maxTime)
		}
	})
}

func TestRetryWaiter_calculateWait(t *testing.T) {
	t.Parallel()

	t.Run("Defaults", func(t *testing.T) {
		t.Parallel()

		rw := NewRetryWaiter(0, 0, 0, nil)

		require.Equal(t, 0*time.Nanosecond, rw.calculateWait())
		rw.failures += 1
		require.Equal(t, 1*time.Second, rw.calculateWait())
		rw.failures += 1
		require.Equal(t, 2*time.Second, rw.calculateWait())
		rw.failures = 31
		require.Equal(t, defaultMaxWait, rw.calculateWait())
	})

	t.Run("Minimum Wait", func(t *testing.T) {
		t.Parallel()

		rw := NewRetryWaiter(0, 5*time.Second, 0, nil)

		require.Equal(t, 5*time.Second, rw.calculateWait())
		rw.failures += 1
		require.Equal(t, 5*time.Second, rw.calculateWait())
		rw.failures += 1
		require.Equal(t, 5*time.Second, rw.calculateWait())
		rw.failures += 1
		require.Equal(t, 5*time.Second, rw.calculateWait())
		rw.failures += 1
		require.Equal(t, 8*time.Second, rw.calculateWait())
	})

	t.Run("Minimum Failures", func(t *testing.T) {
		t.Parallel()

		rw := NewRetryWaiter(5, 0, 0, nil)
		require.Equal(t, 0*time.Nanosecond, rw.calculateWait())
		rw.failures += 5
		require.Equal(t, 0*time.Nanosecond, rw.calculateWait())
		rw.failures += 1
		require.Equal(t, 1*time.Second, rw.calculateWait())
	})

	t.Run("Maximum Wait", func(t *testing.T) {
		t.Parallel()

		rw := NewRetryWaiter(0, 0, 5*time.Second, nil)
		require.Equal(t, 0*time.Nanosecond, rw.calculateWait())
		rw.failures += 1
		require.Equal(t, 1*time.Second, rw.calculateWait())
		rw.failures += 1
		require.Equal(t, 2*time.Second, rw.calculateWait())
		rw.failures += 1
		require.Equal(t, 4*time.Second, rw.calculateWait())
		rw.failures += 1
		require.Equal(t, 5*time.Second, rw.calculateWait())
		rw.failures = 31
		require.Equal(t, 5*time.Second, rw.calculateWait())
	})
}

func TestRetryWaiter_WaitChans(t *testing.T) {
	t.Parallel()

	t.Run("Minimum Wait - Success", func(t *testing.T) {
		t.Parallel()

		rw := NewRetryWaiter(0, 250*time.Millisecond, 0, nil)

		select {
		case <-time.After(200 * time.Millisecond):
		case <-rw.Success():
			require.Fail(t, "minimum wait not respected")
		}
	})

	t.Run("Minimum Wait - WaitIf", func(t *testing.T) {
		t.Parallel()

		rw := NewRetryWaiter(0, 250*time.Millisecond, 0, nil)

		select {
		case <-time.After(200 * time.Millisecond):
		case <-rw.WaitIf(false):
			require.Fail(t, "minimum wait not respected")
		}
	})

	t.Run("Minimum Wait - WaitIfErr", func(t *testing.T) {
		t.Parallel()

		rw := NewRetryWaiter(0, 250*time.Millisecond, 0, nil)

		select {
		case <-time.After(200 * time.Millisecond):
		case <-rw.WaitIfErr(nil):
			require.Fail(t, "minimum wait not respected")
		}
	})

	t.Run("Maximum Wait - Failed", func(t *testing.T) {
		t.Parallel()

		rw := NewRetryWaiter(0, 0, 250*time.Millisecond, nil)

		select {
		case <-time.After(500 * time.Millisecond):
			require.Fail(t, "maximum wait not respected")
		case <-rw.Failed():
		}
	})

	t.Run("Maximum Wait - WaitIf", func(t *testing.T) {
		t.Parallel()

		rw := NewRetryWaiter(0, 0, 250*time.Millisecond, nil)

		select {
		case <-time.After(500 * time.Millisecond):
			require.Fail(t, "maximum wait not respected")
		case <-rw.WaitIf(true):
		}
	})

	t.Run("Maximum Wait - WaitIfErr", func(t *testing.T) {
		t.Parallel()

		rw := NewRetryWaiter(0, 0, 250*time.Millisecond, nil)

		select {
		case <-time.After(500 * time.Millisecond):
			require.Fail(t, "maximum wait not respected")
		case <-rw.WaitIfErr(fmt.Errorf("Fake Error")):
		}
	})
}
