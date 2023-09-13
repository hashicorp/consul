// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package limiter

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSessionLimiter(t *testing.T) {
	lim := NewSessionLimiter()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go lim.Run(ctx)

	// doneCh is used to shut the goroutines down at the end of the test.
	doneCh := make(chan struct{})
	t.Cleanup(func() { close(doneCh) })

	// Start 10 sessions, and increment the counter when they are terminated.
	var (
		terminations uint32
		wg           sync.WaitGroup
	)
	for i := 0; i < 10; i++ {
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

	// Lowering max sessions to 5 should result in 5 sessions being terminated.
	lim.SetMaxSessions(5)
	require.Eventually(t, func() bool {
		return atomic.LoadUint32(&terminations) == 5
	}, 2*time.Second, 50*time.Millisecond)

	// Attempting to start a new session should fail immediately.
	_, err := lim.BeginSession()
	require.Equal(t, ErrCapacityReached, err)

	// Raising MaxSessions should make room for a new session.
	lim.SetMaxSessions(6)
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
