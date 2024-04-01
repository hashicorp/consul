// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package submatview

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/lib/ttlcache"
	"github.com/hashicorp/consul/proto/private/pbcommon"
	"github.com/hashicorp/consul/proto/private/pbservice"
	"github.com/hashicorp/consul/proto/private/pbsubscribe"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

func TestStore_Get(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := NewStore(hclog.New(nil))
	go store.Run(ctx)

	req := &fakeRPCRequest{
		client: NewTestStreamingClient(pbcommon.DefaultEnterpriseMeta.Namespace),
	}
	req.client.QueueEvents(
		newEndOfSnapshotEvent(2),
		newEventServiceHealthRegister(10, 1, "srv1"),
		newEventServiceHealthRegister(22, 2, "srv1"))

	testutil.RunStep(t, "from empty store, starts materializer", func(t *testing.T) {
		var result Result
		retry.Run(t, func(r *retry.R) {
			var err error
			result, err = store.Get(ctx, req)
			require.NoError(r, err)
			require.Equal(r, uint64(22), result.Index)
		})

		r, ok := result.Value.(fakeResult)
		require.True(t, ok)
		require.Len(t, r.srvs, 2)
		require.Equal(t, uint64(22), r.index)

		store.lock.Lock()
		defer store.lock.Unlock()
		require.Len(t, store.byKey, 1)
		e := store.byKey[makeEntryKey(req.Type(), req.CacheInfo())]
		require.Equal(t, 0, e.expiry.Index())
		require.Equal(t, 0, e.requests)

		require.Equal(t, store.expiryHeap.Next().Entry, e.expiry)
	})

	testutil.RunStep(t, "with an index that already exists in the view", func(t *testing.T) {
		req.index = 21
		result, err := store.Get(ctx, req)
		require.NoError(t, err)
		require.Equal(t, uint64(22), result.Index)

		r, ok := result.Value.(fakeResult)
		require.True(t, ok)
		require.Len(t, r.srvs, 2)
		require.Equal(t, uint64(22), r.index)

		store.lock.Lock()
		defer store.lock.Unlock()
		require.Len(t, store.byKey, 1)
		e := store.byKey[makeEntryKey(req.Type(), req.CacheInfo())]
		require.Equal(t, 0, e.expiry.Index())
		require.Equal(t, 0, e.requests)

		require.Equal(t, store.expiryHeap.Next().Entry, e.expiry)
	})

	chResult := make(chan resultOrError, 1)
	req.index = 40
	go func() {
		result, err := store.Get(ctx, req)
		chResult <- resultOrError{Result: result, Err: err}
	}()

	testutil.RunStep(t, "blocks with an index that is not yet in the view", func(t *testing.T) {
		select {
		case <-chResult:
			t.Fatalf("expected Get to block")
		case <-time.After(50 * time.Millisecond):
		}

		store.lock.Lock()
		e := store.byKey[makeEntryKey(req.Type(), req.CacheInfo())]
		store.lock.Unlock()
		require.Equal(t, 1, e.requests)
	})

	testutil.RunStep(t, "blocks when an event is received but the index is still below minIndex", func(t *testing.T) {
		req.client.QueueEvents(newEventServiceHealthRegister(24, 1, "srv1"))

		select {
		case <-chResult:
			t.Fatalf("expected Get to block")
		case <-time.After(50 * time.Millisecond):
		}

		store.lock.Lock()
		e := store.byKey[makeEntryKey(req.Type(), req.CacheInfo())]
		store.lock.Unlock()
		require.Equal(t, 1, e.requests)
	})

	testutil.RunStep(t, "unblocks when an event with index past minIndex", func(t *testing.T) {
		req.client.QueueEvents(newEventServiceHealthRegister(41, 1, "srv1"))
		var getResult resultOrError
		select {
		case getResult = <-chResult:
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("expected Get to unblock when new events are received")
		}

		require.NoError(t, getResult.Err)
		require.Equal(t, uint64(41), getResult.Result.Index)

		r, ok := getResult.Result.Value.(fakeResult)
		require.True(t, ok)
		require.Len(t, r.srvs, 2)
		require.Equal(t, uint64(41), r.index)

		store.lock.Lock()
		defer store.lock.Unlock()
		require.Len(t, store.byKey, 1)
		e := store.byKey[makeEntryKey(req.Type(), req.CacheInfo())]
		require.Equal(t, 0, e.expiry.Index())
		require.Equal(t, 0, e.requests)

		require.Equal(t, store.expiryHeap.Next().Entry, e.expiry)
	})

	testutil.RunStep(t, "with no index returns latest value", func(t *testing.T) {
		req.index = 0
		result, err := store.Get(ctx, req)
		require.NoError(t, err)
		require.Equal(t, uint64(41), result.Index)

		r, ok := result.Value.(fakeResult)
		require.True(t, ok)
		require.Len(t, r.srvs, 2)
		require.Equal(t, uint64(41), r.index)

		store.lock.Lock()
		defer store.lock.Unlock()
		require.Len(t, store.byKey, 1)
		e := store.byKey[makeEntryKey(req.Type(), req.CacheInfo())]
		require.Equal(t, 0, e.expiry.Index())
		require.Equal(t, 0, e.requests)

		require.Equal(t, store.expiryHeap.Next().Entry, e.expiry)
	})

	testutil.RunStep(t, "blocks until timeout", func(t *testing.T) {
		req.index = 50
		req.timeout = 25 * time.Millisecond

		chResult := make(chan resultOrError, 1)
		go func() {
			result, err := store.Get(ctx, req)
			chResult <- resultOrError{Result: result, Err: err}
		}()

		var getResult resultOrError
		select {
		case getResult = <-chResult:
			t.Fatalf("expected Get to block until timeout")
		case <-time.After(10 * time.Millisecond):
		}

		select {
		case getResult = <-chResult:
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("expected Get to unblock after timeout")
		}

		require.NoError(t, getResult.Err)
		require.Equal(t, uint64(41), getResult.Result.Index)

		r, ok := getResult.Result.Value.(fakeResult)
		require.True(t, ok)
		require.Len(t, r.srvs, 2)
		require.Equal(t, uint64(41), r.index)
	})

}

type resultOrError struct {
	Result Result
	Err    error
}

type fakeRPCRequest struct {
	index   uint64
	timeout time.Duration
	key     string
	client  *TestStreamingClient
}

func (r *fakeRPCRequest) CacheInfo() cache.RequestInfo {
	key := r.key
	if key == "" {
		key = "key"
	}
	return cache.RequestInfo{
		Key:        key,
		Token:      "abcd",
		Datacenter: "dc1",
		Timeout:    r.timeout,
		MinIndex:   r.index,
	}
}

func (r *fakeRPCRequest) NewMaterializer() (Materializer, error) {
	deps := Deps{
		View:   &fakeView{srvs: make(map[string]*pbservice.CheckServiceNode)},
		Logger: hclog.New(nil),
		Request: func(index uint64) *pbsubscribe.SubscribeRequest {
			req := &pbsubscribe.SubscribeRequest{
				Topic: pbsubscribe.Topic_ServiceHealth,
				Subject: &pbsubscribe.SubscribeRequest_NamedSubject{
					NamedSubject: &pbsubscribe.NamedSubject{
						Key:       "key",
						Namespace: pbcommon.DefaultEnterpriseMeta.Namespace,
					},
				},
				Token:      "abcd",
				Datacenter: "dc1",
				Index:      index,
			}
			return req
		},
	}
	return NewRPCMaterializer(r.client, deps), nil
}

func (r *fakeRPCRequest) Type() string {
	return fmt.Sprintf("%T", r)
}

type fakeView struct {
	srvs map[string]*pbservice.CheckServiceNode
}

func (f *fakeView) Update(events []*pbsubscribe.Event) error {
	for _, event := range events {
		serviceHealth := event.GetServiceHealth()
		if serviceHealth == nil {
			return fmt.Errorf("unexpected event type for service health view: %T",
				event.GetPayload())
		}

		id := serviceHealth.CheckServiceNode.UniqueID()
		switch serviceHealth.Op {
		case pbsubscribe.CatalogOp_Register:
			f.srvs[id] = serviceHealth.CheckServiceNode

		case pbsubscribe.CatalogOp_Deregister:
			delete(f.srvs, id)
		}
	}
	return nil
}

func (f *fakeView) Result(index uint64) interface{} {
	srvs := make([]*pbservice.CheckServiceNode, 0, len(f.srvs))
	for _, srv := range f.srvs {
		srvs = append(srvs, srv)
	}
	return fakeResult{srvs: srvs, index: index}
}

type fakeResult struct {
	srvs  []*pbservice.CheckServiceNode
	index uint64
}

func (f *fakeView) Reset() {
	f.srvs = make(map[string]*pbservice.CheckServiceNode)
}

func TestStore_Notify(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := NewStore(hclog.New(nil))
	go store.Run(ctx)

	req := &fakeRPCRequest{
		client: NewTestStreamingClient(pbcommon.DefaultEnterpriseMeta.Namespace),
	}
	req.client.QueueEvents(
		newEndOfSnapshotEvent(2),
		newEventServiceHealthRegister(22, 2, "srv1"))

	cID := "correlate"
	ch := make(chan cache.UpdateEvent)

	err := store.Notify(ctx, req, cID, ch)
	require.NoError(t, err)

	testutil.RunStep(t, "from empty store, starts materializer", func(t *testing.T) {
		store.lock.Lock()
		defer store.lock.Unlock()
		require.Len(t, store.byKey, 1)
		e := store.byKey[makeEntryKey(req.Type(), req.CacheInfo())]
		require.Equal(t, ttlcache.NotIndexed, e.expiry.Index())
		require.Equal(t, 1, e.requests)
	})

	testutil.RunStep(t, "updates are received", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			select {
			case update := <-ch:
				require.NoError(r, update.Err)
				require.Equal(r, cID, update.CorrelationID)
				require.Equal(r, uint64(22), update.Meta.Index)
				require.Equal(r, uint64(22), update.Result.(fakeResult).index)
			case <-time.After(100 * time.Millisecond):
				r.Stop(fmt.Errorf("expected Get to unblock when new events are received"))
			}
		})

		req.client.QueueEvents(newEventServiceHealthRegister(24, 2, "srv1"))

		select {
		case update := <-ch:
			require.NoError(t, update.Err)
			require.Equal(t, cID, update.CorrelationID)
			require.Equal(t, uint64(24), update.Meta.Index)
			require.Equal(t, uint64(24), update.Result.(fakeResult).index)
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("expected Get to unblock when new events are received")
		}
	})

	testutil.RunStep(t, "closing the notify starts the expiry counter", func(t *testing.T) {
		cancel()

		retry.Run(t, func(r *retry.R) {
			store.lock.Lock()
			defer store.lock.Unlock()
			e := store.byKey[makeEntryKey(req.Type(), req.CacheInfo())]
			require.Equal(r, 0, e.expiry.Index())
			require.Equal(r, 0, e.requests)
			require.Equal(r, store.expiryHeap.Next().Entry, e.expiry)
		})
	})
}

func TestStore_Notify_ManyRequests(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := NewStore(hclog.New(nil))
	go store.Run(ctx)

	req := &fakeRPCRequest{
		client: NewTestStreamingClient(pbcommon.DefaultEnterpriseMeta.Namespace),
	}
	req.client.QueueEvents(newEndOfSnapshotEvent(2))

	cID := "correlate"
	ch1 := make(chan cache.UpdateEvent)
	ch2 := make(chan cache.UpdateEvent)

	require.NoError(t, store.Notify(ctx, req, cID, ch1))
	assertRequestCount(t, store, req, 1)

	require.NoError(t, store.Notify(ctx, req, cID, ch2))
	assertRequestCount(t, store, req, 2)

	req.index = 15

	go func() {
		_, _ = store.Get(ctx, req)
	}()

	retry.Run(t, func(r *retry.R) {
		assertRequestCount(r, store, req, 3)
	})

	go func() {
		_, _ = store.Get(ctx, req)
	}()

	retry.Run(t, func(r *retry.R) {
		assertRequestCount(r, store, req, 4)
	})

	var req2 *fakeRPCRequest

	testutil.RunStep(t, "Get and Notify with a different key", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		req2 = &fakeRPCRequest{client: req.client, key: "key2", index: 22}

		require.NoError(t, store.Notify(ctx, req2, cID, ch1))
		go func() {
			_, _ = store.Get(ctx, req2)
		}()

		// the original entry should still be at count 4
		assertRequestCount(t, store, req, 4)
		// the new entry should be at count 2
		retry.Run(t, func(r *retry.R) {
			assertRequestCount(r, store, req2, 2)
		})
	})

	testutil.RunStep(t, "end all the requests", func(t *testing.T) {
		req.client.QueueEvents(
			newEventServiceHealthRegister(10, 1, "srv1"),
			newEventServiceHealthRegister(12, 2, "srv1"),
			newEventServiceHealthRegister(13, 1, "srv2"),
			newEventServiceHealthRegister(16, 3, "srv2"))

		// The two Get requests should exit now that the index has been updated
		retry.Run(t, func(r *retry.R) {
			assertRequestCount(r, store, req, 2)
		})

		// Cancel the context so all requests terminate
		cancel()
		retry.Run(t, func(r *retry.R) {
			assertRequestCount(r, store, req, 0)
		})
	})

	testutil.RunStep(t, "the expiry heap should contain two entries", func(t *testing.T) {
		store.lock.Lock()
		defer store.lock.Unlock()
		e := store.byKey[makeEntryKey(req.Type(), req.CacheInfo())]
		e2 := store.byKey[makeEntryKey(req2.Type(), req2.CacheInfo())]
		require.Equal(t, 0, e2.expiry.Index())
		require.Equal(t, 1, e.expiry.Index())

		require.Equal(t, store.expiryHeap.Next().Entry, e2.expiry)
	})
}

type testingT interface {
	Helper()
	Fatalf(string, ...interface{})
}

func assertRequestCount(t testingT, s *Store, req Request, expected int) {
	t.Helper()

	key := makeEntryKey(req.Type(), req.CacheInfo())

	s.lock.Lock()
	defer s.lock.Unlock()
	actual := s.byKey[key].requests
	if actual != expected {
		t.Fatalf("expected request count to be %d, got %d", expected, actual)
	}
}

func TestStore_Run_ExpiresEntries(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := NewStore(hclog.New(nil))
	ttl := 10 * time.Millisecond
	store.idleTTL = ttl
	go store.Run(ctx)

	req := &fakeRPCRequest{
		client: NewTestStreamingClient(pbcommon.DefaultEnterpriseMeta.Namespace),
	}
	req.client.QueueEvents(newEndOfSnapshotEvent(2))

	cID := "correlate"
	ch1 := make(chan cache.UpdateEvent)

	reqCtx, reqCancel := context.WithCancel(context.Background())
	defer reqCancel()

	require.NoError(t, store.Notify(reqCtx, req, cID, ch1))
	assertRequestCount(t, store, req, 1)

	// Get a copy of the entry so that we can check it was expired later
	store.lock.Lock()
	e := store.byKey[makeEntryKey(req.Type(), req.CacheInfo())]
	store.lock.Unlock()

	reqCancel()
	retry.Run(t, func(r *retry.R) {
		assertRequestCount(r, store, req, 0)
	})

	// wait for the entry to expire, with lots of buffer
	time.Sleep(3 * ttl)

	store.lock.Lock()
	defer store.lock.Unlock()
	require.Len(t, store.byKey, 0)
	require.Equal(t, ttlcache.NotIndexed, e.expiry.Index())
}

func TestStore_Run_FailingMaterializer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	store := NewStore(hclog.NewNullLogger())
	store.idleTTL = 24 * time.Hour
	go store.Run(ctx)

	t.Run("with an in-flight request", func(t *testing.T) {
		req := &failingMaterializerRequest{
			doneCh: make(chan struct{}),
		}

		ch := make(chan cache.UpdateEvent)
		reqCtx, reqCancel := context.WithCancel(context.Background())
		t.Cleanup(reqCancel)
		require.NoError(t, store.Notify(reqCtx, req, "", ch))

		assertRequestCount(t, store, req, 1)

		// Cause the materializer to "fail" (exit before its context is canceled).
		close(req.doneCh)

		// End the in-flight request.
		reqCancel()

		// Check that the item was evicted.
		retry.Run(t, func(r *retry.R) {
			store.lock.Lock()
			defer store.lock.Unlock()

			require.Len(r, store.byKey, 0)
		})
	})

	t.Run("with no in-flight requests", func(t *testing.T) {
		req := &failingMaterializerRequest{
			doneCh: make(chan struct{}),
		}

		// Cause the materializer to "fail" (exit before its context is canceled).
		close(req.doneCh)

		// Check that the item was evicted.
		retry.Run(t, func(r *retry.R) {
			store.lock.Lock()
			defer store.lock.Unlock()

			require.Len(r, store.byKey, 0)
		})
	})
}

type failingMaterializerRequest struct {
	doneCh chan struct{}
}

func (failingMaterializerRequest) CacheInfo() cache.RequestInfo { return cache.RequestInfo{} }
func (failingMaterializerRequest) Type() string                 { return "test.FailingMaterializerRequest" }

func (r *failingMaterializerRequest) NewMaterializer() (Materializer, error) {
	return &failingMaterializer{doneCh: r.doneCh}, nil
}

type failingMaterializer struct {
	doneCh <-chan struct{}
}

func (failingMaterializer) Query(context.Context, uint64) (Result, error) { return Result{}, nil }

func (m *failingMaterializer) Run(context.Context) { <-m.doneCh }
