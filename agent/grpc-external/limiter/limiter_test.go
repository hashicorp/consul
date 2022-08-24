package limiter

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLimiter(t *testing.T) {
	termRateLimiter := newTestRateLimiter()
	lim := NewLimiter(termRateLimiter)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go lim.Run(ctx)

	// doneCh is used to shut the goroutines down at the end of the test.
	doneCh := make(chan struct{})
	t.Cleanup(func() { close(doneCh) })

	// Start 18 sessions, and increment the counter when they are terminated.
	var (
		terminations uint32
		wg           sync.WaitGroup
	)
	for i := 0; i < 18; i++ {
		wg.Add(1)
		go func() {
			sess, err := lim.BeginSession()
			require.NoError(t, err)
			defer sess.End()

			wg.Done()

			select {
			case <-sess.Terminated():
				atomic.AddUint32(&terminations, 1)
			case <-doneCh:
			}
		}()
	}

	// Wait for all the sessions to begin.
	wg.Wait()

	// Lowering max sessions to 10 should result in 8 sessions being terminated,
	// but termRateLimiter will only allow 5 to be terminated right now.
	lim.SetMaxSessions(10)
	termRateLimiter.allow(ctx, 5)
	require.Eventually(t, func() bool {
		return atomic.LoadUint32(&terminations) == 5
	}, 2*time.Second, 50*time.Millisecond)

	// Allow the remaining sessions to be terminated.
	termRateLimiter.allow(ctx, 5)
	require.Eventually(t, func() bool {
		return atomic.LoadUint32(&terminations) == 8
	}, 2*time.Second, 50*time.Millisecond)

	// Attempting to start a new session should fail immediately.
	_, err := lim.BeginSession()
	require.Equal(t, ErrCapacityReached, err)

	// Raising MaxSessions should make room for a new session.
	lim.SetMaxSessions(11)
	sess, err := lim.BeginSession()
	require.NoError(t, err)

	// ...but trying to start another new one should fail
	_, err = lim.BeginSession()
	require.Equal(t, ErrCapacityReached, err)

	// ...until another session ends.
	sess.End()
	_, err = lim.BeginSession()
	require.NoError(t, err)

	// Calling End twice is a no-op.
	sess.End()
	_, err = lim.BeginSession()
	require.Equal(t, ErrCapacityReached, err)
}

func newTestRateLimiter() *testWaiter {
	return &testWaiter{waitCh: make(chan struct{})}
}

type testWaiter struct {
	waitCh chan struct{}
}

func (tw *testWaiter) Wait(ctx context.Context) error {
	select {
	case <-tw.waitCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (tw *testWaiter) allow(ctx context.Context, n int) {
	go func() {
		for i := 0; i < n; i++ {
			select {
			case tw.waitCh <- struct{}{}:
			case <-ctx.Done():
				return
			}
		}
	}()
}
