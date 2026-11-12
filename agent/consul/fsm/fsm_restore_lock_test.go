// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: BUSL-1.1

package fsm

import (
	"bytes"
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
