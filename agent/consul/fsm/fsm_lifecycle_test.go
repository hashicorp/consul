// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package fsm

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/lifecycle"
	"github.com/hashicorp/consul/lifecycle/lifecycletest"
)

func newLifecycleFSM(t *testing.T, ex *lifecycle.Exchange) (*FSM, *stream.EventPublisher) {
	t.Helper()
	handle := &testRaftHandle{}
	storageBackend := newStorageBackend(t, handle)
	handle.apply = func(buf []byte) (any, error) { return storageBackend.Apply(buf, 123), nil }

	publisher := stream.NewEventPublisher(10 * time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go publisher.Run(ctx)

	fsm := NewFromDeps(Deps{
		Logger:         hclog.NewNullLogger(),
		NewStateStore:  func() *state.Store { return state.NewStateStore(nil) },
		StorageBackend: storageBackend,
		Publisher:      publisher,
		Lifecycle:      ex,
	})
	return fsm, publisher
}

func persistedSnapshot(t *testing.T, fsm *FSM) []byte {
	t.Helper()
	snap, err := fsm.Snapshot()
	require.NoError(t, err)
	defer snap.Release()
	buf := bytes.NewBuffer(nil)
	require.NoError(t, snap.Persist(&MockSink{buf, false}))
	return buf.Bytes()
}

func webRequest() *stream.SubscribeRequest {
	return &stream.SubscribeRequest{
		Topic:   state.EventTopicServiceHealth,
		Subject: state.EventSubjectService{Key: "web"},
	}
}

func TestFSM_Lifecycle_ServesFromConstructionAndTracksRestore(t *testing.T) {
	rec := &lifecycletest.Recorder{}
	ex := lifecycle.NewExchange(lifecycle.WithObserver(rec))
	t.Cleanup(ex.Close)
	fsm, pub := newLifecycleFSM(t, ex)

	// A fresh server with no snapshot must serve subscriptions straight away.
	st, ok := ex.Status(StateStoreComponent)
	require.True(t, ok)
	require.Equal(t, lifecycle.StateServing, st.State)
	require.Equal(t, uint64(1), st.Generation)

	sub, err := pub.Subscribe(webRequest())
	require.NoError(t, err)
	_, err = sub.Next(context.Background())
	require.NoError(t, err)
	sub.Unsubscribe()

	require.NoError(t, fsm.Restore(io.NopCloser(bytes.NewReader(persistedSnapshot(t, fsm)))))

	st, _ = ex.Status(StateStoreComponent)
	require.Equal(t, lifecycle.StateServing, st.State)
	require.Equal(t, uint64(2), st.Generation, "a restore replaces the state: new generation")
	lifecycletest.AssertTransitions(t, rec, StateStoreComponent,
		lifecycle.StateBooting, lifecycle.StateServing, lifecycle.StateRestoring, lifecycle.StateServing)
	lifecycletest.AssertDrained(t, ex, StateStoreComponent)
}

func TestFSM_Lifecycle_SnapshotsFailFastWhileRestoringAndRestoreWaitsForInFlight(t *testing.T) {
	ex := lifecycle.NewExchange()
	t.Cleanup(ex.Close)
	fsm, pub := newLifecycleFSM(t, ex)
	data := persistedSnapshot(t, fsm)

	// A snapshot that is mid-read holds a lease on the state store.
	inFlight, err := ex.Require(StateStoreComponent, lifecycle.PhaseReady).Acquire(context.Background(), lifecycle.FailFast)
	require.NoError(t, err)

	restored := make(chan error, 1)
	go func() { restored <- fsm.Restore(io.NopCloser(bytes.NewReader(data))) }()

	require.Eventually(t, func() bool {
		st, _ := ex.Status(StateStoreComponent)
		return st.State == lifecycle.StateRestoring
	}, 2*time.Second, time.Millisecond)

	select {
	case err := <-restored:
		t.Fatalf("Restore finished while a snapshot read was in flight: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	// New snapshots are refused, with an error the subscriber sees.
	sub, err := pub.Subscribe(webRequest())
	require.NoError(t, err)
	_, err = sub.Next(context.Background())
	require.Error(t, err)
	require.True(t, lifecycle.IsNotReady(err), "got %v", err)
	sub.Unsubscribe()

	inFlight.Release()
	select {
	case err := <-restored:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Restore did not finish after the in-flight snapshot finished")
	}

	// The failed attempt must not be cached: the next subscriber gets a snapshot.
	sub, err = pub.Subscribe(webRequest())
	require.NoError(t, err)
	defer sub.Unsubscribe()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for {
		ev, err := sub.Next(ctx)
		require.NoError(t, err)
		if ev.IsEndOfSnapshot() {
			break
		}
	}
}

// restoreAgainstSubscribers runs real FSM.Restore calls against many goroutines
// that keep subscribing, and reports whether Restore stopped making progress.
// Subscribe takes the publisher lock and then reads the state through State();
// Restore takes the state lock and then the publisher lock. That is the lock
// order inversion behind silent streaming stalls after a snapshot install.
func restoreAgainstSubscribers(t *testing.T, ex *lifecycle.Exchange, restores int, stall time.Duration) (wedged bool, stacks string) {
	t.Helper()
	fsm, pub := newLifecycleFSM(t, ex)
	data := persistedSnapshot(t, fsm)

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				sub, err := pub.Subscribe(&stream.SubscribeRequest{
					Topic:   state.EventTopicServiceHealth,
					Subject: state.EventSubjectService{Key: fmt.Sprintf("svc-%d", i%4)},
				})
				if err == nil {
					sub.Unsubscribe()
				}
			}
		}(i)
	}

	var done atomic.Int64
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		for i := 0; i < restores; i++ {
			if err := fsm.Restore(io.NopCloser(bytes.NewReader(data))); err != nil {
				t.Errorf("restore: %v", err)
				return
			}
			done.Add(1)
		}
	}()

	last := int64(-1)
	for {
		select {
		case <-finished:
			close(stop)
			wg.Wait()
			return false, ""
		case <-time.After(stall):
			if done.Load() == last {
				buf := make([]byte, 1<<20)
				buf = buf[:runtime.Stack(buf, true)]
				close(stop) // the blocked goroutines leak; the test process exits anyway
				return true, string(buf)
			}
			last = done.Load()
		}
	}
}

// TestFSM_Restore_DoesNotDeadlockWithSubscribe guards the fix for Restore and
// Subscribe deadlocking: with the lifecycle gate, Restore waits for in-flight
// snapshot reads before it takes any lock, so the two can never wait on each
// other whatever order their locks are taken in.
func TestFSM_Restore_DoesNotDeadlockWithSubscribe(t *testing.T) {
	ex := lifecycle.NewExchange()
	t.Cleanup(ex.Close)

	wedged, stacks := restoreAgainstSubscribers(t, ex, 300, 15*time.Second)
	if wedged {
		t.Fatalf("FSM.Restore wedged against concurrent Subscribe calls\n%s", stacks)
	}
}
