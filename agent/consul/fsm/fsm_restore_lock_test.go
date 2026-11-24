// Copyright IBM Corp. 2014, 2026
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
	"github.com/hashicorp/consul/agent/structs"
)

// TestFSM_Restore_DoesNotDeadlockWithSubscribe guards the lock order between
// FSM.Restore and EventPublisher.Subscribe. Subscribe holds the publisher lock
// while its snapshot handler reads the state store through FSM.State(); Restore
// must therefore never hold the state lock while waiting for the publisher lock,
// or a subscription that arrives during a restore wedges the FSM and every
// subscriber of that server, without any error being reported.
func TestFSM_Restore_DoesNotDeadlockWithSubscribe(t *testing.T) {
	handle := &testRaftHandle{}
	storageBackend := newStorageBackend(t, handle)
	handle.apply = func(buf []byte) (any, error) { return storageBackend.Apply(buf, 123), nil }

	publisher := stream.NewEventPublisher(10 * time.Second)
	fsm := NewFromDeps(Deps{
		Logger:         hclog.NewNullLogger(),
		NewStateStore:  func() *state.Store { return state.NewStateStore(nil) },
		StorageBackend: storageBackend,
		Publisher:      publisher,
	})

	snap, err := fsm.Snapshot()
	require.NoError(t, err)
	buf := bytes.NewBuffer(nil)
	require.NoError(t, snap.Persist(&MockSink{buf, false}))
	snap.Release()
	data := buf.Bytes()

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
				sub, err := publisher.Subscribe(&stream.SubscribeRequest{
					Topic:   state.EventTopicServiceHealth,
					Subject: state.EventSubjectService{Key: fmt.Sprintf("svc-%d", i%4)},
				})
				if err == nil {
					sub.Unsubscribe()
				}
			}
		}(i)
	}

	const restores = 300
	var done atomic.Int64
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		for i := 0; i < restores; i++ {
			if err := fsm.Restore(io.NopCloser(bytes.NewReader(data))); err != nil {
				t.Errorf("restore failed: %v", err)
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
			return
		case <-time.After(15 * time.Second):
			if done.Load() == last {
				stacks := make([]byte, 1<<20)
				stacks = stacks[:runtime.Stack(stacks, true)]
				close(stop)
				t.Fatalf("FSM.Restore wedged after %d restores (lock order inversion with Subscribe)\n%s", done.Load(), stacks)
			}
			last = done.Load()
		}
	}
}

func newRestoreTestFSM(t *testing.T, publisher *stream.EventPublisher) *FSM {
	t.Helper()

	handle := &testRaftHandle{}
	storageBackend := newStorageBackend(t, handle)
	handle.apply = func(buf []byte) (any, error) { return storageBackend.Apply(buf, 123), nil }

	deps := Deps{
		Logger:         hclog.NewNullLogger(),
		NewStateStore:  func() *state.Store { return state.NewStateStore(nil) },
		StorageBackend: storageBackend,
	}
	if publisher != nil {
		deps.Publisher = publisher
	}
	return NewFromDeps(deps)
}

func persistTestSnapshot(t *testing.T, fsm *FSM) []byte {
	t.Helper()

	snap, err := fsm.Snapshot()
	require.NoError(t, err)
	defer snap.Release()

	buf := bytes.NewBuffer(nil)
	require.NoError(t, snap.Persist(&MockSink{buf, false}))
	return buf.Bytes()
}

// snapshotServiceIDs reads the snapshot part of a new subscription and returns
// the IDs of the services it contains.
func snapshotServiceIDs(t *testing.T, sub *stream.Subscription) []string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var ids []string
	var visit func(ev stream.Event) bool
	visit = func(ev stream.Event) bool {
		switch p := ev.Payload.(type) {
		case *stream.PayloadEvents:
			for _, item := range p.Items {
				if visit(item) {
					return true
				}
			}
			return false
		case state.EventPayloadCheckServiceNode:
			ids = append(ids, p.Value.Service.ID)
			return false
		default:
			return ev.IsEndOfSnapshot()
		}
	}

	for {
		ev, err := sub.Next(ctx)
		require.NoError(t, err)
		if visit(ev) {
			return ids
		}
	}
}

func newServiceHealthRequest(key string) *stream.SubscribeRequest {
	return &stream.SubscribeRequest{
		Topic:   state.EventTopicServiceHealth,
		Subject: state.EventSubjectService{Key: key},
	}
}

// TestFSM_Restore_ResetsSubscriptionsAndSnapshotCache checks what Restore must
// still guarantee now that the state store is swapped before the publisher is
// told to refresh: existing subscriptions are force-closed so clients
// resubscribe, and the snapshot cached from the old state store is evicted so
// the next subscription reads the restored data.
func TestFSM_Restore_ResetsSubscriptionsAndSnapshotCache(t *testing.T) {
	src := newRestoreTestFSM(t, nil)
	require.NoError(t, src.State().EnsureRegistration(1, &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "node1",
		Address:    "127.0.0.1",
		Service:    &structs.NodeService{ID: "web-1", Service: "web", Port: 80},
	}))
	restored := persistTestSnapshot(t, src)

	publisher := stream.NewEventPublisher(10 * time.Second)
	dst := newRestoreTestFSM(t, publisher)

	// Subscribe before the restore: the snapshot (empty) is built from the old
	// state store and cached.
	before, err := publisher.Subscribe(newServiceHealthRequest("web"))
	require.NoError(t, err)
	t.Cleanup(before.Unsubscribe)
	require.Empty(t, snapshotServiceIDs(t, before))

	require.NoError(t, dst.Restore(io.NopCloser(bytes.NewReader(restored))))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err = before.Next(ctx)
	require.ErrorIs(t, err, stream.ErrSubForceClosed)

	after, err := publisher.Subscribe(newServiceHealthRequest("web"))
	require.NoError(t, err)
	t.Cleanup(after.Unsubscribe)
	require.Equal(t, []string{"web-1"}, snapshotServiceIDs(t, after))
}

// TestFSM_Restore_NoStaleSnapshotAfterRestore hammers Subscribe while the FSM
// alternates between restoring a snapshot with and without a service. Once a
// Restore returns, any new subscription must see the restored data: a snapshot
// built from the previous state store must never survive in the cache, even
// though the state store is swapped before the publisher refreshes.
func TestFSM_Restore_NoStaleSnapshotAfterRestore(t *testing.T) {
	src := newRestoreTestFSM(t, nil)
	empty := persistTestSnapshot(t, src)
	require.NoError(t, src.State().EnsureRegistration(1, &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "node1",
		Address:    "127.0.0.1",
		Service:    &structs.NodeService{ID: "web-1", Service: "web", Port: 80},
	}))
	withWeb := persistTestSnapshot(t, src)

	publisher := stream.NewEventPublisher(10 * time.Second)
	dst := newRestoreTestFSM(t, publisher)

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				sub, err := publisher.Subscribe(newServiceHealthRequest("web"))
				if err == nil {
					sub.Unsubscribe()
				}
			}
		}()
	}
	defer func() {
		close(stop)
		wg.Wait()
	}()

	for i := 0; i < 200; i++ {
		data, want := empty, []string(nil)
		if i%2 == 1 {
			data, want = withWeb, []string{"web-1"}
		}
		require.NoError(t, dst.Restore(io.NopCloser(bytes.NewReader(data))))

		sub, err := publisher.Subscribe(newServiceHealthRequest("web"))
		require.NoError(t, err)
		got := snapshotServiceIDs(t, sub)
		sub.Unsubscribe()
		require.Equal(t, want, got, "restore %d: subscription got a snapshot from before the restore", i)
	}
}
