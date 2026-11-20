// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: BUSL-1.1

package lifecycle

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-metrics"
	"github.com/stretchr/testify/require"
)

func TestComponent_StartingReadyAndGeneration(t *testing.T) {
	c := NewComponent("server/state-store")
	require.Equal(t, PhaseStarting, c.Status().Phase)

	_, err := c.Acquire()
	require.Error(t, err)
	require.True(t, IsNotReady(err))
	var nr *NotReadyError
	require.ErrorAs(t, err, &nr)
	require.Equal(t, "server/state-store", nr.Component)
	require.Equal(t, PhaseStarting, nr.Phase)

	c.Ready("Serving", "created")
	st := c.Status()
	require.Equal(t, PhaseReady, st.Phase)
	require.Equal(t, "Serving", st.State)
	require.Equal(t, uint64(1), st.Generation)
	require.Equal(t, "created", st.Reason)

	// Ready only applies to a starting component.
	c.Ready("Other", "again")
	require.Equal(t, "Serving", c.Status().State)

	release, err := c.Acquire()
	require.NoError(t, err)
	release()
	release() // idempotent
	require.Zero(t, c.leaseCount())
}

func (c *Component) leaseCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.leases
}

func TestComponent_BeginTransitionRefusesNewLeasesAndDrainsExisting(t *testing.T) {
	c := NewComponent("c")
	c.Ready("Serving", "")

	held, err := c.Acquire()
	require.NoError(t, err)

	type result struct {
		finish func(string, string)
		err    error
	}
	began := make(chan result, 1)
	go func() {
		finish, err := c.BeginTransition(context.Background(), "Restoring", "snapshot")
		began <- result{finish, err}
	}()

	require.Eventually(t, func() bool { return c.Status().Phase == PhaseTransitioning }, 2*time.Second, time.Millisecond)

	// New leases are refused while draining.
	_, err = c.Acquire()
	require.True(t, IsNotReady(err))
	require.Contains(t, err.Error(), "Restoring")

	// A second transition is refused.
	_, err = c.BeginTransition(context.Background(), "Restoring", "again")
	require.ErrorIs(t, err, ErrTransitionInProgress)

	select {
	case <-began:
		t.Fatal("BeginTransition returned while a lease was held")
	case <-time.After(50 * time.Millisecond):
	}

	held()
	r := <-began
	require.NoError(t, r.err)

	// Still refused until the transition is finished.
	_, err = c.Acquire()
	require.True(t, IsNotReady(err))

	r.finish("Serving", "done")
	r.finish("Serving", "ignored") // once only

	st := c.Status()
	require.Equal(t, PhaseReady, st.Phase)
	require.Equal(t, uint64(2), st.Generation, "a replacement makes a new generation")
	require.Equal(t, "done", st.Reason)

	release, err := c.Acquire()
	require.NoError(t, err)
	release()
}

func TestComponent_BeginTransitionWithoutLeasesDoesNotWait(t *testing.T) {
	c := NewComponent("c")
	c.Ready("Serving", "")
	finish, err := c.BeginTransition(context.Background(), "Restoring", "")
	require.NoError(t, err)
	finish("Serving", "")
	require.Equal(t, uint64(2), c.Status().Generation)
}

func TestComponent_DrainTimeoutProceedsAndWarns(t *testing.T) {
	var logs bytes.Buffer
	logger := hclog.New(&hclog.LoggerOptions{Output: &logs, Level: hclog.Warn})
	c := NewComponent("c", WithLogger(logger), WithDrainTimeout(30*time.Millisecond))
	c.Ready("Serving", "")

	held, err := c.Acquire()
	require.NoError(t, err)

	start := time.Now()
	finish, err := c.BeginTransition(context.Background(), "Restoring", "")
	require.NoError(t, err)
	require.GreaterOrEqual(t, time.Since(start), 30*time.Millisecond)
	require.Contains(t, logs.String(), "proceeding with leases still held")

	finish("Serving", "")
	held() // a late release must not break anything
	require.Zero(t, c.leaseCount())
	release, err := c.Acquire()
	require.NoError(t, err)
	release()
}

func TestComponent_DrainEndsWithContext(t *testing.T) {
	c := NewComponent("c", WithDrainTimeout(time.Minute))
	c.Ready("Serving", "")
	held, err := c.Acquire()
	require.NoError(t, err)
	defer held()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	finish, err := c.BeginTransition(ctx, "Restoring", "")
	require.NoError(t, err)
	finish("Serving", "")
}

func TestComponent_NilIsANoop(t *testing.T) {
	var c *Component
	c.Ready("Serving", "")
	require.Equal(t, Status{}, c.Status())
	release, err := c.Acquire()
	require.NoError(t, err)
	release()
	finish, err := c.BeginTransition(context.Background(), "Restoring", "")
	require.NoError(t, err)
	finish("Serving", "")
}

func TestNotReadyError(t *testing.T) {
	err := &NotReadyError{Component: "c", State: "Restoring", Phase: PhaseTransitioning, Reason: "snapshot"}
	require.Equal(t, "c is not ready: Restoring (transitioning): snapshot", err.Error())
	require.True(t, IsNotReady(errors.Join(errors.New("outer"), err)))
	require.False(t, IsNotReady(errors.New("other")))
}

// TestComponent_PreventsLockOrderDeadlock models the bug this package exists
// for with two plain locks taken in opposite orders:
//
//	consumer: lock A, then (inside a lease) read-lock B
//	owner:    BeginTransition, lock B, then lock A
//
// Without the lease and drain this deadlocks. With them the owner only takes B
// once no consumer can be waiting for it.
func TestComponent_PreventsLockOrderDeadlock(t *testing.T) {
	c := NewComponent("c")
	c.Ready("Serving", "")

	var a sync.Mutex
	var b sync.RWMutex
	var served, refused, reads, swaps atomic.Int64

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				a.Lock()
				release, err := c.Acquire()
				if err != nil {
					refused.Add(1)
					a.Unlock()
					continue
				}
				b.RLock()
				reads.Add(1) // stands in for reading the state store
				b.RUnlock()
				release()
				served.Add(1)
				a.Unlock()
			}
		}()
	}

	restores := 0
	for ; restores < 500 && ctx.Err() == nil; restores++ {
		finish, err := c.BeginTransition(ctx, "Restoring", "")
		require.NoError(t, err)
		b.Lock()
		a.Lock()
		swaps.Add(1) // stands in for swapping the state and refreshing the publisher
		a.Unlock()
		b.Unlock()
		finish("Serving", "")
	}
	cancel()
	wg.Wait()

	require.Equal(t, 500, restores, "restores stalled: deadlock")
	require.NotZero(t, served.Load())
	require.Equal(t, served.Load(), reads.Load())
	require.Equal(t, int64(restores), swaps.Load())
	t.Logf("restores=%d served=%d refused=%d", restores, served.Load(), refused.Load())
}

func TestComponent_LeaseStress(t *testing.T) {
	c := NewComponent("c")
	c.Ready("Serving", "")

	var inside atomic.Int64
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				release, err := c.Acquire()
				if err != nil {
					continue
				}
				inside.Add(1)
				inside.Add(-1)
				release()
			}
		}()
	}
	for i := 0; i < 1000; i++ {
		finish, err := c.BeginTransition(ctx, "Restoring", "")
		require.NoError(t, err)
		require.Zero(t, inside.Load(), "lease holders inside after drain")
		require.Zero(t, c.leaseCount(), "leases after drain")
		finish("Serving", "")
	}
	cancel()
	wg.Wait()
	require.Zero(t, c.leaseCount())
}

func TestComponent_EmitsMetrics(t *testing.T) {
	sink := metrics.NewInmemSink(10*time.Second, time.Minute)
	cfg := metrics.DefaultConfig("consul")
	cfg.EnableHostname = false
	cfg.EnableRuntimeMetrics = false
	_, err := metrics.NewGlobal(cfg, sink)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = metrics.NewGlobal(metrics.DefaultConfig("consul"), &metrics.BlackholeSink{}) })

	c := NewComponent("server/state-store")
	c.Ready("Serving", "")
	finish, err := c.BeginTransition(context.Background(), "Restoring", "")
	require.NoError(t, err)
	finish("Serving", "")

	counter := func(key string) int {
		n := 0
		for _, iv := range sink.Data() {
			iv.RLock()
			if s, ok := iv.Counters[key]; ok {
				n += s.Count
			}
			iv.RUnlock()
		}
		return n
	}
	require.Equal(t, 1, counter("consul.lifecycle.transition;component=server/state-store;from=ready;to=transitioning"))
	require.Equal(t, 1, counter("consul.lifecycle.transition;component=server/state-store;from=transitioning;to=ready"))

	var phase float32 = -1
	for _, iv := range sink.Data() {
		iv.RLock()
		if g, ok := iv.Gauges["consul.lifecycle.phase;component=server/state-store"]; ok {
			phase = g.Value
		}
		iv.RUnlock()
	}
	require.Equal(t, float32(PhaseReady), phase)
}

func TestDefinitionsCoverEmittedMetrics(t *testing.T) {
	names := map[string]bool{}
	for _, d := range Gauges {
		names[d.Name[1]] = true
	}
	for _, d := range Counters {
		names[d.Name[1]] = true
	}
	for _, want := range []string{"phase", "transition", "drain_timeout"} {
		require.True(t, names[want], "missing definition for consul.lifecycle.%s", want)
	}
}
