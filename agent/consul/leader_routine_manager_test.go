package consul

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/stretchr/testify/require"
)

func TestLeaderRoutineManager(t *testing.T) {
	t.Parallel()
	var runs uint32
	var running uint32
	// tlog := testutil.NewCancellableTestLogger(t)
	// defer tlog.Cancel()
	mgr := NewLeaderRoutineManager(testutil.Logger(t))

	run := func(ctx context.Context) error {
		atomic.StoreUint32(&running, 1)
		defer atomic.StoreUint32(&running, 0)
		atomic.AddUint32(&runs, 1)
		<-ctx.Done()
		return nil
	}

	// IsRunning on unregistered service should be false
	require.False(t, mgr.IsRunning("not-found"))

	// start
	require.NoError(t, mgr.Start("run", run))
	require.True(t, mgr.IsRunning("run"))
	retry.Run(t, func(r *retry.R) {
		require.Equal(r, uint32(1), atomic.LoadUint32(&runs))
		require.Equal(r, uint32(1), atomic.LoadUint32(&running))
	})
	require.NoError(t, mgr.Stop("run"))

	// ensure the background go routine was actually cancelled
	retry.Run(t, func(r *retry.R) {
		require.Equal(r, uint32(1), atomic.LoadUint32(&runs))
		require.Equal(r, uint32(0), atomic.LoadUint32(&running))
	})

	// restart and stop
	require.NoError(t, mgr.Start("run", run))
	retry.Run(t, func(r *retry.R) {
		require.Equal(r, uint32(2), atomic.LoadUint32(&runs))
		require.Equal(r, uint32(1), atomic.LoadUint32(&running))
	})

	require.NoError(t, mgr.Stop("run"))
	retry.Run(t, func(r *retry.R) {
		require.Equal(r, uint32(0), atomic.LoadUint32(&running))
	})

	// start with a context
	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, mgr.StartWithContext(ctx, "run", run))
	cancel()

	// The function should exit of its own accord due to the parent
	// context being canceled
	retry.Run(t, func(r *retry.R) {
		require.Equal(r, uint32(3), atomic.LoadUint32(&runs))
		require.Equal(r, uint32(0), atomic.LoadUint32(&running))
		// the task should automatically set itself to not running if
		// it exits early
		require.False(r, mgr.IsRunning("run"))
	})
}
