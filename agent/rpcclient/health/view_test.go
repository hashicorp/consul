// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package health

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/submatview"
	"github.com/hashicorp/consul/proto/private/pbcommon"
	"github.com/hashicorp/consul/proto/private/pbservice"
	"github.com/hashicorp/consul/proto/private/pbsubscribe"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/types"
)

func TestSortCheckServiceNodes_OrderIsConsistentWithRPCResponse(t *testing.T) {
	index := uint64(42)
	buildTestNode := func(nodeName string, serviceID string) structs.CheckServiceNode {
		newID, err := uuid.GenerateUUID()
		require.NoError(t, err)
		return structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         types.NodeID(strings.ToUpper(newID)),
				Node:       nodeName,
				Address:    nodeName,
				Datacenter: "dc1",
				RaftIndex: structs.RaftIndex{
					CreateIndex: index,
					ModifyIndex: index,
				},
			},
			Service: &structs.NodeService{
				ID:      serviceID,
				Service: "testService",
				Port:    8080,
				Weights: &structs.Weights{
					Passing: 1,
					Warning: 1,
				},
				RaftIndex: structs.RaftIndex{
					CreateIndex: index,
					ModifyIndex: index,
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			},
			Checks: []*structs.HealthCheck{},
		}
	}
	zero := buildTestNode("a-zero-node", "testService:1")
	one := buildTestNode("node1", "testService:1")
	two := buildTestNode("node1", "testService:2")
	three := buildTestNode("node2", "testService")
	result := structs.IndexedCheckServiceNodes{
		Nodes:     structs.CheckServiceNodes{three, two, zero, one},
		QueryMeta: structs.QueryMeta{Index: index},
	}
	sortCheckServiceNodes(&result)
	expected := structs.CheckServiceNodes{zero, one, two, three}
	require.Equal(t, expected, result.Nodes)
}

func TestHealthView_IntegrationWithStore_WithEmptySnapshot(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("local data", func(t *testing.T) {
		testHealthView_IntegrationWithStore_WithEmptySnapshot(t, structs.DefaultPeerKeyword)
	})

	t.Run("peered data", func(t *testing.T) {
		testHealthView_IntegrationWithStore_WithEmptySnapshot(t, "my-peer")
	})
}

func testHealthView_IntegrationWithStore_WithEmptySnapshot(t *testing.T, peerName string) {
	namespace := getNamespace(pbcommon.DefaultEnterpriseMeta.Namespace)
	streamClient := newStreamClient(validateNamespace(namespace))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := submatview.NewStore(hclog.New(nil))
	go store.Run(ctx)

	// Initially there are no services registered. Server should send an
	// EndOfSnapshot message immediately with index of 1.
	streamClient.QueueEvents(newEndOfSnapshotEvent(1))

	req := serviceRequestStub{
		serviceRequest: serviceRequest{
			ServiceSpecificRequest: structs.ServiceSpecificRequest{
				PeerName:       peerName,
				Datacenter:     "dc1",
				ServiceName:    "web",
				EnterpriseMeta: structs.NewEnterpriseMetaInDefaultPartition(namespace),
				QueryOptions:   structs.QueryOptions{MaxQueryTime: time.Second},
			},
		},
		streamClient: streamClient,
	}
	empty := &structs.IndexedCheckServiceNodes{
		Nodes: structs.CheckServiceNodes{},
		QueryMeta: structs.QueryMeta{
			Index:   1,
			Backend: structs.QueryBackendStreaming,
		},
	}

	testutil.RunStep(t, "empty snapshot returned", func(t *testing.T) {
		result, err := store.Get(ctx, req)
		require.NoError(t, err)

		require.Equal(t, uint64(1), result.Index)
		require.Equal(t, empty, result.Value)

		req.QueryOptions.MinQueryIndex = result.Index
	})

	testutil.RunStep(t, "blocks for timeout", func(t *testing.T) {
		// Subsequent fetch should block for the timeout
		start := time.Now()
		req.QueryOptions.MaxQueryTime = 200 * time.Millisecond
		result, err := store.Get(ctx, req)
		require.NoError(t, err)
		elapsed := time.Since(start)
		require.True(t, elapsed >= 200*time.Millisecond,
			"Fetch should have blocked until timeout")

		require.Equal(t, req.QueryOptions.MinQueryIndex, result.Index, "result index should not have changed")
		require.Equal(t, empty, result.Value, "result value should not have changed")

		req.QueryOptions.MinQueryIndex = result.Index
	})

	var lastResultValue structs.CheckServiceNodes

	testutil.RunStep(t, "blocks until update", func(t *testing.T) {
		// Make another blocking query with a longer timeout and trigger an update
		// event part way through.
		start := time.Now()
		go func() {
			time.Sleep(200 * time.Millisecond)
			streamClient.QueueEvents(newEventServiceHealthRegister(4, 1, "web", peerName))
		}()

		req.QueryOptions.MaxQueryTime = time.Second
		result, err := store.Get(ctx, req)
		require.NoError(t, err)
		elapsed := time.Since(start)
		require.True(t, elapsed >= 200*time.Millisecond,
			"Fetch should have blocked until the event was delivered")
		require.True(t, elapsed < time.Second,
			"Fetch should have returned before the timeout")

		require.Equal(t, uint64(4), result.Index, "result index should not have changed")
		lastResultValue = result.Value.(*structs.IndexedCheckServiceNodes).Nodes
		require.Len(t, lastResultValue, 1,
			"result value should contain the new registration")

		require.Equal(t, peerName, lastResultValue[0].Node.PeerName)
		require.Equal(t, peerName, lastResultValue[0].Service.PeerName)

		req.QueryOptions.MinQueryIndex = result.Index
	})

	testutil.RunStep(t, "reconnects and resumes after temporary error", func(t *testing.T) {
		streamClient.QueueErr(tempError("broken pipe"))

		// Next fetch will continue to block until timeout and receive the same
		// result.
		start := time.Now()
		req.QueryOptions.MaxQueryTime = 200 * time.Millisecond
		result, err := store.Get(ctx, req)
		require.NoError(t, err)
		elapsed := time.Since(start)
		require.True(t, elapsed >= 200*time.Millisecond,
			"Fetch should have blocked until timeout")

		require.Equal(t, req.QueryOptions.MinQueryIndex, result.Index,
			"result index should not have changed")
		require.Equal(t, lastResultValue, result.Value.(*structs.IndexedCheckServiceNodes).Nodes,
			"result value should not have changed")

		req.QueryOptions.MinQueryIndex = result.Index

		// But an update should still be noticed due to reconnection
		streamClient.QueueEvents(newEventServiceHealthRegister(10, 2, "web", peerName))

		start = time.Now()
		req.QueryOptions.MaxQueryTime = time.Second
		result, err = store.Get(ctx, req)
		require.NoError(t, err)
		elapsed = time.Since(start)
		require.True(t, elapsed < time.Second,
			"Fetch should have returned before the timeout")

		require.Equal(t, uint64(10), result.Index, "result index should not have changed")
		lastResultValue = result.Value.(*structs.IndexedCheckServiceNodes).Nodes
		require.Len(t, lastResultValue, 2,
			"result value should contain the new registration")

		require.Equal(t, peerName, lastResultValue[0].Node.PeerName)
		require.Equal(t, peerName, lastResultValue[0].Service.PeerName)
		require.Equal(t, peerName, lastResultValue[1].Node.PeerName)
		require.Equal(t, peerName, lastResultValue[1].Service.PeerName)

		req.QueryOptions.MinQueryIndex = result.Index
	})

	testutil.RunStep(t, "returns non-temporary error to watchers", func(t *testing.T) {
		// Wait and send the error while fetcher is waiting
		go func() {
			time.Sleep(200 * time.Millisecond)
			streamClient.QueueErr(errors.New("invalid request"))
		}()

		// Next fetch should return the error
		start := time.Now()
		req.QueryOptions.MaxQueryTime = time.Second
		result, err := store.Get(ctx, req)
		require.Error(t, err)
		elapsed := time.Since(start)
		require.True(t, elapsed >= 200*time.Millisecond,
			"Fetch should have blocked until error was sent")
		require.True(t, elapsed < time.Second,
			"Fetch should have returned before the timeout")

		require.Equal(t, req.QueryOptions.MinQueryIndex, result.Index, "result index should not have changed")
		require.Equal(t, lastResultValue, result.Value.(*structs.IndexedCheckServiceNodes).Nodes)

		req.QueryOptions.MinQueryIndex = result.Index

		// But an update should still be noticed due to reconnection
		streamClient.QueueEvents(newEventServiceHealthRegister(req.QueryOptions.MinQueryIndex+5, 3, "web", peerName))

		req.QueryOptions.MaxQueryTime = time.Second
		result, err = store.Get(ctx, req)
		require.NoError(t, err)
		elapsed = time.Since(start)
		require.True(t, elapsed < time.Second, "Fetch should have returned before the timeout")

		require.Equal(t, req.QueryOptions.MinQueryIndex+5, result.Index, "result index should not have changed")
		lastResultValue = result.Value.(*structs.IndexedCheckServiceNodes).Nodes
		require.Len(t, lastResultValue, 3,
			"result value should contain the new registration")

		require.Equal(t, peerName, lastResultValue[0].Node.PeerName)
		require.Equal(t, peerName, lastResultValue[0].Service.PeerName)
		require.Equal(t, peerName, lastResultValue[1].Node.PeerName)
		require.Equal(t, peerName, lastResultValue[1].Service.PeerName)
		require.Equal(t, peerName, lastResultValue[2].Node.PeerName)
		require.Equal(t, peerName, lastResultValue[2].Service.PeerName)

		req.QueryOptions.MinQueryIndex = result.Index
	})
}

type tempError string

func (e tempError) Error() string {
	return string(e)
}

func (e tempError) Temporary() bool {
	return true
}

func TestHealthView_IntegrationWithStore_WithFullSnapshot(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("local data", func(t *testing.T) {
		testHealthView_IntegrationWithStore_WithFullSnapshot(t, structs.DefaultPeerKeyword)
	})

	t.Run("peered data", func(t *testing.T) {
		testHealthView_IntegrationWithStore_WithFullSnapshot(t, "my-peer")
	})
}

func testHealthView_IntegrationWithStore_WithFullSnapshot(t *testing.T, peerName string) {
	namespace := getNamespace("ns2")
	client := newStreamClient(validateNamespace(namespace))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := submatview.NewStore(hclog.New(nil))

	// Create an initial snapshot of 3 instances on different nodes
	registerServiceWeb := func(index uint64, nodeNum int) *pbsubscribe.Event {
		return newEventServiceHealthRegister(index, nodeNum, "web", peerName)
	}
	client.QueueEvents(
		registerServiceWeb(5, 1),
		registerServiceWeb(5, 2),
		registerServiceWeb(5, 3),
		newEndOfSnapshotEvent(5))

	req := serviceRequestStub{
		serviceRequest: serviceRequest{
			ServiceSpecificRequest: structs.ServiceSpecificRequest{
				PeerName:       peerName,
				Datacenter:     "dc1",
				ServiceName:    "web",
				EnterpriseMeta: structs.NewEnterpriseMetaInDefaultPartition(namespace),
				QueryOptions:   structs.QueryOptions{MaxQueryTime: time.Second},
			},
		},
		streamClient: client,
	}

	testutil.RunStep(t, "full snapshot returned", func(t *testing.T) {
		result, err := store.Get(ctx, req)
		require.NoError(t, err)

		require.Equal(t, uint64(5), result.Index)
		expected := newExpectedNodesInPeer(peerName, "node1", "node2", "node3")
		expected.Index = 5
		prototest.AssertDeepEqual(t, expected, result.Value, cmpCheckServiceNodeNames)

		req.QueryOptions.MinQueryIndex = result.Index
	})

	testutil.RunStep(t, "blocks until deregistration", func(t *testing.T) {
		// Make another blocking query with a longer timeout and trigger an update
		// event part way through.
		start := time.Now()
		go func() {
			time.Sleep(200 * time.Millisecond)

			// Deregister instance on node1
			client.QueueEvents(newEventServiceHealthDeregister(20, 1, "web", peerName))
		}()

		req.QueryOptions.MaxQueryTime = time.Second
		result, err := store.Get(ctx, req)
		require.NoError(t, err)
		elapsed := time.Since(start)
		require.True(t, elapsed >= 200*time.Millisecond,
			"Fetch should have blocked until the event was delivered")
		require.True(t, elapsed < time.Second,
			"Fetch should have returned before the timeout")

		require.Equal(t, uint64(20), result.Index)
		expected := newExpectedNodesInPeer(peerName, "node2", "node3")
		expected.Index = 20
		prototest.AssertDeepEqual(t, expected, result.Value, cmpCheckServiceNodeNames)

		req.QueryOptions.MinQueryIndex = result.Index
	})

	testutil.RunStep(t, "server reload is respected", func(t *testing.T) {
		// Simulates the server noticing the request's ACL token privs changing. To
		// detect this we'll queue up the new snapshot as a different set of nodes
		// to the first.
		client.QueueErr(status.Error(codes.Aborted, "reset by server"))

		client.QueueEvents(
			registerServiceWeb(50, 3), // overlap existing node
			registerServiceWeb(50, 4),
			registerServiceWeb(50, 5),
			newEndOfSnapshotEvent(50))

		// Make another blocking query with THE SAME index. It should immediately
		// return the new snapshot.
		start := time.Now()
		req.QueryOptions.MaxQueryTime = time.Second
		result, err := store.Get(ctx, req)
		require.NoError(t, err)
		elapsed := time.Since(start)
		require.True(t, elapsed < time.Second,
			"Fetch should have returned before the timeout")

		require.Equal(t, uint64(50), result.Index)
		expected := newExpectedNodesInPeer(peerName, "node3", "node4", "node5")
		expected.Index = 50
		prototest.AssertDeepEqual(t, expected, result.Value, cmpCheckServiceNodeNames)

		req.QueryOptions.MinQueryIndex = result.Index
	})

	testutil.RunStep(t, "reconnects and receives new snapshot when server state has changed", func(t *testing.T) {
		client.QueueErr(tempError("temporary connection error"))

		client.QueueEvents(
			newNewSnapshotToFollowEvent(),
			registerServiceWeb(50, 3), // overlap existing node
			registerServiceWeb(50, 4),
			registerServiceWeb(50, 5),
			newEndOfSnapshotEvent(50))

		start := time.Now()
		req.QueryOptions.MinQueryIndex = 49
		req.QueryOptions.MaxQueryTime = time.Second
		result, err := store.Get(ctx, req)
		require.NoError(t, err)
		elapsed := time.Since(start)
		require.True(t, elapsed < time.Second,
			"Fetch should have returned before the timeout")

		require.Equal(t, uint64(50), result.Index)
		expected := newExpectedNodesInPeer(peerName, "node3", "node4", "node5")
		expected.Index = 50
		prototest.AssertDeepEqual(t, expected, result.Value, cmpCheckServiceNodeNames)
	})
}

func newExpectedNodesInPeer(peerName string, nodes ...string) *structs.IndexedCheckServiceNodes {
	result := &structs.IndexedCheckServiceNodes{}
	result.QueryMeta.Backend = structs.QueryBackendStreaming
	for _, node := range nodes {
		result.Nodes = append(result.Nodes, structs.CheckServiceNode{
			Node: &structs.Node{
				Node:     node,
				PeerName: peerName,
			},
		})
	}
	return result
}

// cmpCheckServiceNodeNames does a shallow comparison of structs.CheckServiceNode
// by Node name.
var cmpCheckServiceNodeNames = cmp.Options{
	cmp.Comparer(func(x, y structs.CheckServiceNode) bool {
		return x.Node.Node == y.Node.Node
	}),
}

func TestHealthView_IntegrationWithStore_EventBatches(t *testing.T) {
	t.Run("local data", func(t *testing.T) {
		testHealthView_IntegrationWithStore_EventBatches(t, structs.DefaultPeerKeyword)
	})

	t.Run("peered data", func(t *testing.T) {
		testHealthView_IntegrationWithStore_EventBatches(t, "my-peer")
	})
}

func testHealthView_IntegrationWithStore_EventBatches(t *testing.T, peerName string) {
	namespace := getNamespace("ns3")
	client := newStreamClient(validateNamespace(namespace))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := submatview.NewStore(hclog.New(nil))

	// Create an initial snapshot of 3 instances but in a single event batch
	batchEv := newEventBatchWithEvents(
		newEventServiceHealthRegister(5, 1, "web", peerName),
		newEventServiceHealthRegister(5, 2, "web", peerName),
		newEventServiceHealthRegister(5, 3, "web", peerName))
	client.QueueEvents(
		batchEv,
		newEndOfSnapshotEvent(5))

	req := serviceRequestStub{
		serviceRequest: serviceRequest{
			ServiceSpecificRequest: structs.ServiceSpecificRequest{
				PeerName:       peerName,
				Datacenter:     "dc1",
				ServiceName:    "web",
				EnterpriseMeta: structs.NewEnterpriseMetaInDefaultPartition(namespace),
				QueryOptions:   structs.QueryOptions{MaxQueryTime: time.Second},
			},
		},
		streamClient: client,
	}

	testutil.RunStep(t, "full snapshot returned", func(t *testing.T) {
		result, err := store.Get(ctx, req)
		require.NoError(t, err)

		require.Equal(t, uint64(5), result.Index)

		expected := newExpectedNodesInPeer(peerName, "node1", "node2", "node3")
		expected.Index = 5
		prototest.AssertDeepEqual(t, expected, result.Value, cmpCheckServiceNodeNames)
		req.QueryOptions.MinQueryIndex = result.Index
	})

	testutil.RunStep(t, "batched updates work too", func(t *testing.T) {
		// Simulate multiple registrations happening in one Txn (so all have same
		// index)
		batchEv := newEventBatchWithEvents(
			// Deregister an existing node
			newEventServiceHealthDeregister(20, 1, "web", peerName),
			// Register another
			newEventServiceHealthRegister(20, 4, "web", peerName),
		)
		client.QueueEvents(batchEv)
		req.QueryOptions.MaxQueryTime = time.Second
		result, err := store.Get(ctx, req)
		require.NoError(t, err)

		require.Equal(t, uint64(20), result.Index)
		expected := newExpectedNodesInPeer(peerName, "node2", "node3", "node4")
		expected.Index = 20
		prototest.AssertDeepEqual(t, expected, result.Value, cmpCheckServiceNodeNames)

		req.QueryOptions.MinQueryIndex = result.Index
	})
}

func TestHealthView_IntegrationWithStore_Filtering(t *testing.T) {
	t.Run("local data", func(t *testing.T) {
		testHealthView_IntegrationWithStore_Filtering(t, structs.DefaultPeerKeyword)
	})

	t.Run("peered data", func(t *testing.T) {
		testHealthView_IntegrationWithStore_Filtering(t, "my-peer")
	})
}

func testHealthView_IntegrationWithStore_Filtering(t *testing.T, peerName string) {
	namespace := getNamespace("ns3")
	streamClient := newStreamClient(validateNamespace(namespace))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := submatview.NewStore(hclog.New(nil))
	go store.Run(ctx)

	req := serviceRequestStub{
		serviceRequest: serviceRequest{
			ServiceSpecificRequest: structs.ServiceSpecificRequest{
				PeerName:       peerName,
				Datacenter:     "dc1",
				ServiceName:    "web",
				EnterpriseMeta: structs.NewEnterpriseMetaInDefaultPartition(namespace),
				QueryOptions: structs.QueryOptions{
					Filter:       `Node.Node == "node2"`,
					MaxQueryTime: time.Second,
				},
			},
		},
		streamClient: streamClient,
	}

	// Create an initial snapshot of 3 instances but in a single event batch
	batchEv := newEventBatchWithEvents(
		newEventServiceHealthRegister(5, 1, "web", peerName),
		newEventServiceHealthRegister(5, 2, "web", peerName),
		newEventServiceHealthRegister(5, 3, "web", peerName))
	streamClient.QueueEvents(
		batchEv,
		newEndOfSnapshotEvent(5))

	testutil.RunStep(t, "filtered snapshot returned", func(t *testing.T) {
		result, err := store.Get(ctx, req)
		require.NoError(t, err)

		require.Equal(t, uint64(5), result.Index)
		expected := newExpectedNodesInPeer(peerName, "node2")
		expected.Index = 5
		prototest.AssertDeepEqual(t, expected, result.Value, cmpCheckServiceNodeNames)

		req.QueryOptions.MinQueryIndex = result.Index
	})

	testutil.RunStep(t, "filtered updates work too", func(t *testing.T) {
		// Simulate multiple registrations happening in one Txn (all have same index)
		batchEv := newEventBatchWithEvents(
			// Deregister an existing node
			newEventServiceHealthDeregister(20, 1, "web", peerName),
			// Register another
			newEventServiceHealthRegister(20, 4, "web", peerName),
		)
		streamClient.QueueEvents(batchEv)
		result, err := store.Get(ctx, req)
		require.NoError(t, err)

		require.Equal(t, uint64(20), result.Index)
		expected := newExpectedNodesInPeer(peerName, "node2")
		expected.Index = 20
		prototest.AssertDeepEqual(t, expected, result.Value, cmpCheckServiceNodeNames)
	})
}

// serviceRequestStub overrides NewMaterializer so that test can use a fake
// StreamClient.
type serviceRequestStub struct {
	serviceRequest
	streamClient submatview.StreamClient
}

func (r serviceRequestStub) NewMaterializer() (submatview.Materializer, error) {
	view, err := NewHealthView(r.ServiceSpecificRequest)
	if err != nil {
		return nil, err
	}
	deps := submatview.Deps{
		View:    view,
		Logger:  hclog.New(nil),
		Request: NewMaterializerRequest(r.ServiceSpecificRequest),
	}
	return submatview.NewRPCMaterializer(r.streamClient, deps), nil
}

func newEventServiceHealthRegister(index uint64, nodeNum int, svc string, peerName string) *pbsubscribe.Event {
	node := fmt.Sprintf("node%d", nodeNum)
	nodeID := types.NodeID(fmt.Sprintf("11111111-2222-3333-4444-%012d", nodeNum))
	addr := fmt.Sprintf("10.10.%d.%d", nodeNum/256, nodeNum%256)

	return &pbsubscribe.Event{
		Index: index,
		Payload: &pbsubscribe.Event_ServiceHealth{
			ServiceHealth: &pbsubscribe.ServiceHealthUpdate{
				Op: pbsubscribe.CatalogOp_Register,
				CheckServiceNode: &pbservice.CheckServiceNode{
					Node: &pbservice.Node{
						ID:         string(nodeID),
						Node:       node,
						Address:    addr,
						Datacenter: "dc1",
						PeerName:   peerName,
						RaftIndex: &pbcommon.RaftIndex{
							CreateIndex: index,
							ModifyIndex: index,
						},
					},
					Service: &pbservice.NodeService{
						ID:       svc,
						Service:  svc,
						PeerName: peerName,
						Port:     8080,
						RaftIndex: &pbcommon.RaftIndex{
							CreateIndex: index,
							ModifyIndex: index,
						},
					},
				},
			},
		},
	}
}

func newEventServiceHealthDeregister(index uint64, nodeNum int, svc string, peerName string) *pbsubscribe.Event {
	node := fmt.Sprintf("node%d", nodeNum)

	return &pbsubscribe.Event{
		Index: index,
		Payload: &pbsubscribe.Event_ServiceHealth{
			ServiceHealth: &pbsubscribe.ServiceHealthUpdate{
				Op: pbsubscribe.CatalogOp_Deregister,
				CheckServiceNode: &pbservice.CheckServiceNode{
					Node: &pbservice.Node{
						Node:     node,
						PeerName: peerName,
					},
					Service: &pbservice.NodeService{
						ID:       svc,
						Service:  svc,
						PeerName: peerName,
						Port:     8080,
						Weights: &pbservice.Weights{
							Passing: 1,
							Warning: 1,
						},
						RaftIndex: &pbcommon.RaftIndex{
							// The original insertion index since a delete doesn't update
							// this. This magic value came from state store tests where we
							// setup at index 10 and then mutate at index 100. It can be
							// modified by the caller later and makes it easier than having
							// yet another argument in the common case.
							CreateIndex: 10,
							ModifyIndex: 10,
						},
					},
				},
			},
		},
	}
}

func newEventBatchWithEvents(first *pbsubscribe.Event, evs ...*pbsubscribe.Event) *pbsubscribe.Event {
	events := make([]*pbsubscribe.Event, len(evs)+1)
	events[0] = first
	for i := range evs {
		events[i+1] = evs[i]
	}
	return &pbsubscribe.Event{
		Index: first.Index,
		Payload: &pbsubscribe.Event_EventBatch{
			EventBatch: &pbsubscribe.EventBatch{Events: events},
		},
	}
}

func newEndOfSnapshotEvent(index uint64) *pbsubscribe.Event {
	return &pbsubscribe.Event{
		Index:   index,
		Payload: &pbsubscribe.Event_EndOfSnapshot{EndOfSnapshot: true},
	}
}

func newNewSnapshotToFollowEvent() *pbsubscribe.Event {
	return &pbsubscribe.Event{
		Payload: &pbsubscribe.Event_NewSnapshotToFollow{NewSnapshotToFollow: true},
	}
}

// getNamespace returns a namespace if namespace support exists, otherwise
// returns the empty string. It allows the same tests to work in both ce and ent
// without duplicating the tests.
func getNamespace(ns string) string {
	meta := structs.NewEnterpriseMetaInDefaultPartition(ns)
	return meta.NamespaceOrEmpty()
}

func validateNamespace(ns string) func(request *pbsubscribe.SubscribeRequest) error {
	return func(request *pbsubscribe.SubscribeRequest) error {
		if got := request.GetNamedSubject().GetNamespace(); got != ns {
			return fmt.Errorf("expected request.NamedSubject.Namespace %v, got %v", ns, got)
		}
		return nil
	}
}

func TestNewFilterEvaluator(t *testing.T) {
	type testCase struct {
		name     string
		req      structs.ServiceSpecificRequest
		data     structs.CheckServiceNode
		expected bool
	}

	fn := func(t *testing.T, tc testCase) {
		e, err := newFilterEvaluator(tc.req)
		require.NoError(t, err)
		actual, err := e.Evaluate(tc.data)
		require.NoError(t, err)
		require.Equal(t, tc.expected, actual)
	}

	var testCases = []testCase{
		{
			name: "single ServiceTags match",
			req: structs.ServiceSpecificRequest{
				ServiceTags: []string{"match"},
				TagFilter:   true,
			},
			data: structs.CheckServiceNode{
				Service: &structs.NodeService{
					Tags: []string{"extra", "match"},
				},
			},
			expected: true,
		},
		{
			name: "single deprecated ServiceTag match",
			req: structs.ServiceSpecificRequest{
				ServiceTag: "match",
				TagFilter:  true,
			},
			data: structs.CheckServiceNode{
				Service: &structs.NodeService{
					Tags: []string{"extra", "match"},
				},
			},
			expected: true,
		},
		{
			name: "single ServiceTags mismatch",
			req: structs.ServiceSpecificRequest{
				ServiceTags: []string{"other"},
				TagFilter:   true,
			},
			data: structs.CheckServiceNode{
				Service: &structs.NodeService{
					Tags: []string{"extra", "match"},
				},
			},
			expected: false,
		},
		{
			name: "multiple ServiceTags match",
			req: structs.ServiceSpecificRequest{
				ServiceTags: []string{"match", "second"},
				TagFilter:   true,
			},
			data: structs.CheckServiceNode{
				Service: &structs.NodeService{
					Tags: []string{"extra", "match", "second"},
				},
			},
			expected: true,
		},
		{
			name: "multiple ServiceTags mismatch",
			req: structs.ServiceSpecificRequest{
				ServiceTags: []string{"match", "not"},
				TagFilter:   true,
			},
			data: structs.CheckServiceNode{
				Service: &structs.NodeService{
					Tags: []string{"extra", "match"},
				},
			},
			expected: false,
		},
		{
			name: "single NodeMetaFilter match",
			req: structs.ServiceSpecificRequest{
				NodeMetaFilters: map[string]string{"meta1": "match"},
			},
			data: structs.CheckServiceNode{
				Node: &structs.Node{
					Meta: map[string]string{
						"meta1": "match",
						"extra": "some",
					},
				},
			},
			expected: true,
		},
		{
			name: "single NodeMetaFilter mismatch",
			req: structs.ServiceSpecificRequest{
				NodeMetaFilters: map[string]string{
					"meta1": "match",
				},
			},
			data: structs.CheckServiceNode{
				Node: &structs.Node{
					Meta: map[string]string{
						"meta1": "other",
						"extra": "some",
					},
				},
			},
			expected: false,
		},
		{
			name: "multiple NodeMetaFilter match",
			req: structs.ServiceSpecificRequest{
				NodeMetaFilters: map[string]string{"meta1": "match", "meta2": "a"},
			},
			data: structs.CheckServiceNode{
				Node: &structs.Node{
					Meta: map[string]string{
						"meta1": "match",
						"meta2": "a",
						"extra": "some",
					},
				},
			},
			expected: true,
		},
		{
			name: "multiple NodeMetaFilter mismatch",
			req: structs.ServiceSpecificRequest{
				NodeMetaFilters: map[string]string{
					"meta1": "match",
					"meta2": "beta",
				},
			},
			data: structs.CheckServiceNode{
				Node: &structs.Node{
					Meta: map[string]string{
						"meta1": "other",
						"meta2": "gamma",
					},
				},
			},
			expected: false,
		},
		{
			name: "QueryOptions.Filter match",
			req: structs.ServiceSpecificRequest{
				QueryOptions: structs.QueryOptions{
					Filter: `Node.Node == "node3"`,
				},
			},
			data: structs.CheckServiceNode{
				Node: &structs.Node{Node: "node3"},
			},
			expected: true,
		},
		{
			name: "QueryOptions.Filter mismatch",
			req: structs.ServiceSpecificRequest{
				QueryOptions: structs.QueryOptions{
					Filter: `Node.Node == "node2"`,
				},
			},
			data: structs.CheckServiceNode{
				Node: &structs.Node{Node: "node3"},
			},
			expected: false,
		},
		{
			name: "all match",
			req: structs.ServiceSpecificRequest{
				QueryOptions: structs.QueryOptions{
					Filter: `Node.Node == "node3"`,
				},
				ServiceTags: []string{"tag1", "tag2"},
				NodeMetaFilters: map[string]string{
					"meta1": "match1",
					"meta2": "match2",
				},
			},
			data: structs.CheckServiceNode{
				Node: &structs.Node{
					Node: "node3",
					Meta: map[string]string{
						"meta1": "match1",
						"meta2": "match2",
						"extra": "other",
					},
				},
				Service: &structs.NodeService{
					Tags: []string{"tag1", "tag2", "extra"},
				},
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fn(t, tc)
		})
	}
}

func TestHealthView_SkipFilteringTerminatingGateways(t *testing.T) {
	view, err := NewHealthView(structs.ServiceSpecificRequest{
		ServiceName: "name",
		Connect:     true,
		QueryOptions: structs.QueryOptions{
			Filter: "Service.Meta.version == \"v1\"",
		},
	})
	require.NoError(t, err)

	err = view.Update([]*pbsubscribe.Event{{
		Index: 1,
		Payload: &pbsubscribe.Event_ServiceHealth{
			ServiceHealth: &pbsubscribe.ServiceHealthUpdate{
				Op: pbsubscribe.CatalogOp_Register,
				CheckServiceNode: &pbservice.CheckServiceNode{
					Service: &pbservice.NodeService{
						Kind:    structs.TerminatingGateway,
						Service: "name",
						Address: "127.0.0.1",
						Port:    8443,
					},
				},
			},
		},
	}})
	require.NoError(t, err)

	node, ok := (view.Result(1)).(*structs.IndexedCheckServiceNodes)
	require.True(t, ok)

	require.Len(t, node.Nodes, 1)
	require.Equal(t, "127.0.0.1", node.Nodes[0].Service.Address)
	require.Equal(t, 8443, node.Nodes[0].Service.Port)
}

func TestConfigEntryListView_Reset(t *testing.T) {
	emptyMap := make(map[string]structs.CheckServiceNode)
	view := &HealthView{state: map[string]structs.CheckServiceNode{
		"test": {},
	}}
	view.Reset()
	require.Equal(t, emptyMap, view.state)
}
