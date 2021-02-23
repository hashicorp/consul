package submatview

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/proto/pbcommon"
	"github.com/hashicorp/consul/proto/pbservice"
	"github.com/hashicorp/consul/proto/pbsubscribe"
)

func TestStore_Get_Fresh(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := NewStore()
	go store.Run(ctx)

	req := &fakeRequest{
		client: NewTestStreamingClient(pbcommon.DefaultEnterpriseMeta.Namespace),
	}
	req.client.QueueEvents(
		newEndOfSnapshotEvent(2),
		newEventServiceHealthRegister(10, 1, "srv1"),
		newEventServiceHealthRegister(22, 2, "srv1"))

	result, md, err := store.Get(ctx, req)
	require.NoError(t, err)
	require.Equal(t, uint64(22), md.Index)

	r, ok := result.(fakeResult)
	require.True(t, ok)
	require.Len(t, r.srvs, 2)
	require.Equal(t, uint64(22), r.index)

	store.lock.Lock()
	require.Len(t, store.byKey, 1)
	e := store.byKey[makeEntryKey(req.Type(), req.CacheInfo())]
	require.Equal(t, 0, e.expiry.Index())

	defer store.lock.Unlock()
	require.Equal(t, store.expiryHeap.Next().Entry, e.expiry)
}

type fakeRequest struct {
	client *TestStreamingClient
}

func (r *fakeRequest) CacheInfo() cache.RequestInfo {
	return cache.RequestInfo{
		Key:        "key",
		Token:      "abcd",
		Datacenter: "dc1",
		Timeout:    4 * time.Second,
	}
}

func (r *fakeRequest) NewMaterializer() *Materializer {
	return NewMaterializer(Deps{
		View:   &fakeView{srvs: make(map[string]*pbservice.CheckServiceNode)},
		Client: r.client,
		Logger: hclog.New(nil),
		Request: func(index uint64) pbsubscribe.SubscribeRequest {
			req := pbsubscribe.SubscribeRequest{
				Topic:      pbsubscribe.Topic_ServiceHealth,
				Key:        "key",
				Token:      "abcd",
				Datacenter: "dc1",
				Index:      index,
				Namespace:  pbcommon.DefaultEnterpriseMeta.Namespace,
			}
			return req
		},
	})
}

func (r *fakeRequest) Type() string {
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

// TODO: Get with an entry that already has index
// TODO: Get with an entry that is not yet at index

func TestStore_Notify(t *testing.T) {
	// TODO: Notify with no existing entry
	// TODO: Notify with Get
	// TODO: Notify multiple times same key
	// TODO: Notify no update if index is not past MinIndex.
}
