// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package routine

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/stretchr/testify/require"
)

func TestManager(t *testing.T) {
	t.Parallel()
	var runs uint32
	var running uint32
	mgr := NewManager(testutil.Logger(t))

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
	require.NoError(t, mgr.Start(context.Background(), "run", run))
	require.True(t, mgr.IsRunning("run"))
	retry.Run(t, func(r *retry.R) {
		require.Equal(r, uint32(1), atomic.LoadUint32(&runs))
		require.Equal(r, uint32(1), atomic.LoadUint32(&running))
	})
	doneCh := mgr.Stop("run")
	require.NotNil(t, doneCh)
	<-doneCh

	// ensure the background go routine was actually cancelled
	retry.Run(t, func(r *retry.R) {
		require.Equal(r, uint32(1), atomic.LoadUint32(&runs))
		require.Equal(r, uint32(0), atomic.LoadUint32(&running))
	})

	// restart and stop
	require.NoError(t, mgr.Start(context.Background(), "run", run))
	retry.Run(t, func(r *retry.R) {
		require.Equal(r, uint32(2), atomic.LoadUint32(&runs))
		require.Equal(r, uint32(1), atomic.LoadUint32(&running))
	})

	doneCh = mgr.Stop("run")
	require.NotNil(t, doneCh)
	<-doneCh

	retry.Run(t, func(r *retry.R) {
		require.Equal(r, uint32(0), atomic.LoadUint32(&running))
	})

	// start with a context
	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, mgr.Start(ctx, "run", run))
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

func TestManager_StopAll(t *testing.T) {
	t.Parallel()
	var runs uint32
	var running uint32
	mgr := NewManager(testutil.Logger(t))

	run := func(ctx context.Context) error {
		atomic.StoreUint32(&running, 1)
		defer atomic.StoreUint32(&running, 0)
		atomic.AddUint32(&runs, 1)
		<-ctx.Done()
		return nil
	}

	require.NoError(t, mgr.Start(context.Background(), "run1", run))
	require.NoError(t, mgr.Start(context.Background(), "run2", run))

	mgr.StopAll()

	retry.Run(t, func(r *retry.R) {
		require.False(r, mgr.IsRunning("run1"))
		require.False(r, mgr.IsRunning("run2"))
	})
}

// Test IsRunning when routine is a blocking call that does not
// immediately return when cancelled
func TestManager_StopBlocking(t *testing.T) {
	t.Parallel()
	var runs uint32
	var running uint32
	unblock := make(chan struct{}) // To simulate a blocking call
	mgr := NewManager(testutil.Logger(t))

	// A routine that will be still running for a while after cancelled
	run := func(ctx context.Context) error {
		atomic.StoreUint32(&running, 1)
		defer atomic.StoreUint32(&running, 0)
		atomic.AddUint32(&runs, 1)
		<-ctx.Done()
		<-unblock
		return nil
	}

	require.NoError(t, mgr.Start(context.Background(), "blocking", run))
	retry.Run(t, func(r *retry.R) {
		require.True(r, mgr.IsRunning("blocking"))
		require.Equal(r, uint32(1), atomic.LoadUint32(&runs))
		require.Equal(r, uint32(1), atomic.LoadUint32(&running))
	})

	doneCh := mgr.Stop("blocking")

	// IsRunning should return false, however &running is still 1
	retry.Run(t, func(r *retry.R) {
		require.False(r, mgr.IsRunning("blocking"))
		require.Equal(r, uint32(1), atomic.LoadUint32(&running))
	})

	// New routine should be able to replace old "cancelled but running" routine.
	require.NoError(t, mgr.Start(context.Background(), "blocking", func(ctx context.Context) error {
		<-ctx.Done()
		return nil
	}))
	defer mgr.Stop("blocking")

	retry.Run(t, func(r *retry.R) {
		require.True(r, mgr.IsRunning("blocking"))               // New routine
		require.Equal(r, uint32(1), atomic.LoadUint32(&running)) // Old routine
	})

	// Complete the blocking routine
	close(unblock)
	<-doneCh

	retry.Run(t, func(r *retry.R) {
		require.Equal(r, uint32(0), atomic.LoadUint32(&running))
	})
}
