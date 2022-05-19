package peering

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbpeering"
	"github.com/hashicorp/consul/proto/pbservice"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestSubscriptionManager_RegisterDeregister(t *testing.T) {
	backend := newTestSubscriptionBackend(t)
	// initialCatalogIdx := backend.lastIdx

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a peering
	_, id := backend.ensurePeering(t, "my-peering")
	partition := acl.DefaultEnterpriseMeta().PartitionOrEmpty()

	mgr := newSubscriptionManager(ctx, testutil.Logger(t), backend)
	subCh := mgr.subscribe(ctx, id, partition)

	var (
		gatewayCorrID = subMeshGateway + partition

		mysqlCorrID = subExportedService + structs.NewServiceName("mysql", nil).String()

		mysqlProxyCorrID = subExportedService + structs.NewServiceName("mysql-sidecar-proxy", nil).String()
	)

	// Expect just the empty mesh gateway event to replicate.
	expectEvents(t, subCh, func(t *testing.T, got cache.UpdateEvent) {
		checkEvent(t, got, gatewayCorrID, 0)
	})

	testutil.RunStep(t, "initial export syncs empty instance lists", func(t *testing.T) {
		backend.ensureConfigEntry(t, &structs.ExportedServicesConfigEntry{
			Name: "default",
			Services: []structs.ExportedService{
				{
					Name: "mysql",
					Consumers: []structs.ServiceConsumer{
						{PeerName: "my-peering"},
					},
				},
				{
					Name: "mongo",
					Consumers: []structs.ServiceConsumer{
						{PeerName: "my-other-peering"},
					},
				},
			},
		})

		expectEvents(t, subCh,
			func(t *testing.T, got cache.UpdateEvent) {
				checkEvent(t, got, mysqlCorrID, 0)
			},
			func(t *testing.T, got cache.UpdateEvent) {
				checkEvent(t, got, mysqlProxyCorrID, 0)
			},
		)
	})

	mysql1 := &structs.CheckServiceNode{
		Node:    &structs.Node{Node: "foo", Address: "10.0.0.1"},
		Service: &structs.NodeService{ID: "mysql-1", Service: "mysql", Port: 5000},
		Checks: structs.HealthChecks{
			&structs.HealthCheck{CheckID: "mysql-check", ServiceID: "mysql-1", Node: "foo"},
		},
	}

	testutil.RunStep(t, "registering exported service instance yields update", func(t *testing.T) {
		backend.ensureNode(t, mysql1.Node)
		backend.ensureService(t, "foo", mysql1.Service)

		// We get one update for the service
		expectEvents(t, subCh, func(t *testing.T, got cache.UpdateEvent) {
			require.Equal(t, mysqlCorrID, got.CorrelationID)
			res := got.Result.(*pbservice.IndexedCheckServiceNodes)
			require.Equal(t, uint64(0), res.Index)

			require.Len(t, res.Nodes, 1)
			node := res.Nodes[0]
			require.NotNil(t, node.Node)
			require.Equal(t, "foo", node.Node.Node)
			require.NotNil(t, node.Service)
			require.Equal(t, "mysql-1", node.Service.ID)
			require.Len(t, node.Checks, 0)
		})

		backend.ensureCheck(t, mysql1.Checks[0])

		// and one for the check
		expectEvents(t, subCh, func(t *testing.T, got cache.UpdateEvent) {
			require.Equal(t, mysqlCorrID, got.CorrelationID)
			res := got.Result.(*pbservice.IndexedCheckServiceNodes)
			require.Equal(t, uint64(0), res.Index)

			require.Len(t, res.Nodes, 1)
			node := res.Nodes[0]
			require.NotNil(t, node.Node)
			require.Equal(t, "foo", node.Node.Node)
			require.NotNil(t, node.Service)
			require.Equal(t, "mysql-1", node.Service.ID)
			require.Len(t, node.Checks, 1)
			require.Equal(t, "mysql-1:overall-check", node.Checks[0].CheckID)
		})
	})

	mysql2 := &structs.CheckServiceNode{
		Node:    &structs.Node{Node: "bar", Address: "10.0.0.2"},
		Service: &structs.NodeService{ID: "mysql-2", Service: "mysql", Port: 5000},
		Checks: structs.HealthChecks{
			&structs.HealthCheck{CheckID: "mysql-2-check", ServiceID: "mysql-2", Node: "bar"},
		},
	}

	testutil.RunStep(t, "additional instances are returned when registered", func(t *testing.T) {
		backend.ensureNode(t, mysql2.Node)
		backend.ensureService(t, "bar", mysql2.Service)

		// We get one update for the service
		expectEvents(t, subCh, func(t *testing.T, got cache.UpdateEvent) {
			require.Equal(t, mysqlCorrID, got.CorrelationID)
			res := got.Result.(*pbservice.IndexedCheckServiceNodes)
			require.Equal(t, uint64(0), res.Index)

			require.Len(t, res.Nodes, 2)
			{
				node := res.Nodes[0]
				require.NotNil(t, node.Node)
				require.Equal(t, "bar", node.Node.Node)
				require.NotNil(t, node.Service)
				require.Equal(t, "mysql-2", node.Service.ID)
				require.Len(t, node.Checks, 0)
			}
			{
				node := res.Nodes[1]
				require.NotNil(t, node.Node)
				require.Equal(t, "foo", node.Node.Node)
				require.NotNil(t, node.Service)
				require.Len(t, node.Checks, 1)
				require.Equal(t, "mysql-1:overall-check", node.Checks[0].CheckID)
			}
		})

		backend.ensureCheck(t, mysql2.Checks[0])

		// and one for the check
		expectEvents(t, subCh, func(t *testing.T, got cache.UpdateEvent) {
			require.Equal(t, mysqlCorrID, got.CorrelationID)
			res := got.Result.(*pbservice.IndexedCheckServiceNodes)
			require.Equal(t, uint64(0), res.Index)

			require.Len(t, res.Nodes, 2)
			{
				node := res.Nodes[0]
				require.NotNil(t, node.Node)
				require.Equal(t, "bar", node.Node.Node)
				require.NotNil(t, node.Service)
				require.Equal(t, "mysql-2", node.Service.ID)
				require.Len(t, node.Checks, 1)
				require.Equal(t, "mysql-2:overall-check", node.Checks[0].CheckID)
			}
			{
				node := res.Nodes[1]
				require.NotNil(t, node.Node)
				require.Equal(t, "foo", node.Node.Node)
				require.NotNil(t, node.Service)
				require.Len(t, node.Checks, 1)
				require.Equal(t, "mysql-1:overall-check", node.Checks[0].CheckID)
			}
		})
	})

	mongo := &structs.CheckServiceNode{
		Node:    &structs.Node{Node: "zip", Address: "10.0.0.3"},
		Service: &structs.NodeService{ID: "mongo", Service: "mongo", Port: 5000},
		Checks: structs.HealthChecks{
			&structs.HealthCheck{CheckID: "mongo-check", ServiceID: "mongo", Node: "zip"},
		},
	}

	testutil.RunStep(t, "no updates are received for services not exported to my-peering", func(t *testing.T) {
		backend.ensureNode(t, mongo.Node)
		backend.ensureService(t, "zip", mongo.Service)
		backend.ensureCheck(t, mongo.Checks[0])

		// Receive from subCh times out.
		expectEvents(t, subCh)
	})

	testutil.RunStep(t, "deregister an instance and it gets removed from the output", func(t *testing.T) {
		backend.deleteService(t, "foo", mysql1.Service.ID)

		expectEvents(t, subCh, func(t *testing.T, got cache.UpdateEvent) {
			require.Equal(t, mysqlCorrID, got.CorrelationID)
			res := got.Result.(*pbservice.IndexedCheckServiceNodes)
			require.Equal(t, uint64(0), res.Index)

			require.Len(t, res.Nodes, 1)

			node := res.Nodes[0]
			require.NotNil(t, node.Node)
			require.Equal(t, "bar", node.Node.Node)
			require.NotNil(t, node.Service)
			require.Equal(t, "mysql-2", node.Service.ID)
			require.Len(t, node.Checks, 1)
			require.Equal(t, "mysql-2:overall-check", node.Checks[0].CheckID)
		})
	})

	testutil.RunStep(t, "deregister the last instance and the output is empty", func(t *testing.T) {
		backend.deleteService(t, "bar", mysql2.Service.ID)

		expectEvents(t, subCh, func(t *testing.T, got cache.UpdateEvent) {
			require.Equal(t, mysqlCorrID, got.CorrelationID)
			res := got.Result.(*pbservice.IndexedCheckServiceNodes)
			require.Equal(t, uint64(0), res.Index)

			require.Len(t, res.Nodes, 0)
		})
	})
}

func TestSubscriptionManager_InitialSnapshot(t *testing.T) {
	backend := newTestSubscriptionBackend(t)
	// initialCatalogIdx := backend.lastIdx

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a peering
	_, id := backend.ensurePeering(t, "my-peering")
	partition := acl.DefaultEnterpriseMeta().PartitionOrEmpty()

	mgr := newSubscriptionManager(ctx, testutil.Logger(t), backend)
	subCh := mgr.subscribe(ctx, id, partition)

	// Register two services that are not yet exported
	mysql := &structs.CheckServiceNode{
		Node:    &structs.Node{Node: "foo", Address: "10.0.0.1"},
		Service: &structs.NodeService{ID: "mysql-1", Service: "mysql", Port: 5000},
	}
	backend.ensureNode(t, mysql.Node)
	backend.ensureService(t, "foo", mysql.Service)

	mongo := &structs.CheckServiceNode{
		Node:    &structs.Node{Node: "zip", Address: "10.0.0.3"},
		Service: &structs.NodeService{ID: "mongo-1", Service: "mongo", Port: 5000},
	}
	backend.ensureNode(t, mongo.Node)
	backend.ensureService(t, "zip", mongo.Service)

	var (
		gatewayCorrID = subMeshGateway + partition

		mysqlCorrID = subExportedService + structs.NewServiceName("mysql", nil).String()
		mongoCorrID = subExportedService + structs.NewServiceName("mongo", nil).String()
		chainCorrID = subExportedService + structs.NewServiceName("chain", nil).String()

		mysqlProxyCorrID = subExportedService + structs.NewServiceName("mysql-sidecar-proxy", nil).String()
		mongoProxyCorrID = subExportedService + structs.NewServiceName("mongo-sidecar-proxy", nil).String()
		chainProxyCorrID = subExportedService + structs.NewServiceName("chain-sidecar-proxy", nil).String()
	)

	// Expect just the empty mesh gateway event to replicate.
	expectEvents(t, subCh, func(t *testing.T, got cache.UpdateEvent) {
		checkEvent(t, got, gatewayCorrID, 0)
	})

	// At this point in time we'll have a mesh-gateway notification with no
	// content stored and handled.
	testutil.RunStep(t, "exporting the two services yields an update for both", func(t *testing.T) {
		backend.ensureConfigEntry(t, &structs.ExportedServicesConfigEntry{
			Name: "default",
			Services: []structs.ExportedService{
				{
					Name: "mysql",
					Consumers: []structs.ServiceConsumer{
						{PeerName: "my-peering"},
					},
				},
				{
					Name: "mongo",
					Consumers: []structs.ServiceConsumer{
						{PeerName: "my-peering"},
					},
				},
				{
					Name: "chain",
					Consumers: []structs.ServiceConsumer{
						{PeerName: "my-peering"},
					},
				},
			},
		})

		expectEvents(t, subCh,
			func(t *testing.T, got cache.UpdateEvent) {
				checkEvent(t, got, chainCorrID, 0)
			},
			func(t *testing.T, got cache.UpdateEvent) {
				checkEvent(t, got, chainProxyCorrID, 0)
			},
			func(t *testing.T, got cache.UpdateEvent) {
				checkEvent(t, got, mongoCorrID, 1, "mongo", string(structs.ServiceKindTypical))
			},
			func(t *testing.T, got cache.UpdateEvent) {
				checkEvent(t, got, mongoProxyCorrID, 0)
			},
			func(t *testing.T, got cache.UpdateEvent) {
				checkEvent(t, got, mysqlCorrID, 1, "mysql", string(structs.ServiceKindTypical))
			},
			func(t *testing.T, got cache.UpdateEvent) {
				checkEvent(t, got, mysqlProxyCorrID, 0)
			},
		)
	})

	testutil.RunStep(t, "registering a mesh gateway triggers connect replies", func(t *testing.T) {
		gateway := &structs.CheckServiceNode{
			Node:    &structs.Node{Node: "mgw", Address: "10.1.1.1"},
			Service: &structs.NodeService{ID: "gateway-1", Kind: structs.ServiceKindMeshGateway, Service: "gateway", Port: 8443},
			// TODO: checks
		}
		backend.ensureNode(t, gateway.Node)
		backend.ensureService(t, "mgw", gateway.Service)

		expectEvents(t, subCh,
			func(t *testing.T, got cache.UpdateEvent) {
				checkEvent(t, got, chainProxyCorrID, 1, "chain-sidecar-proxy", string(structs.ServiceKindConnectProxy))
			},
			func(t *testing.T, got cache.UpdateEvent) {
				checkEvent(t, got, mongoProxyCorrID, 1, "mongo-sidecar-proxy", string(structs.ServiceKindConnectProxy))
			},
			func(t *testing.T, got cache.UpdateEvent) {
				checkEvent(t, got, mysqlProxyCorrID, 1, "mysql-sidecar-proxy", string(structs.ServiceKindConnectProxy))
			},
			func(t *testing.T, got cache.UpdateEvent) {
				checkEvent(t, got, gatewayCorrID, 1, "gateway", string(structs.ServiceKindMeshGateway))
			},
		)
	})
}

type testSubscriptionBackend struct {
	state.EventPublisher
	store *state.Store

	lastIdx uint64
}

func newTestSubscriptionBackend(t *testing.T) *testSubscriptionBackend {
	publisher := stream.NewEventPublisher(10 * time.Second)
	store := newStateStore(t, publisher)

	backend := &testSubscriptionBackend{
		EventPublisher: publisher,
		store:          store,
	}

	// Create some placeholder data to ensure raft index > 0
	//
	// TODO(peering): is there some extremely subtle max-index table reading bug in play?
	placeholder := &structs.CheckServiceNode{
		Node:    &structs.Node{Node: "placeholder", Address: "10.0.0.1"},
		Service: &structs.NodeService{ID: "placeholder-1", Service: "placeholder", Port: 5000},
	}
	backend.ensureNode(t, placeholder.Node)
	backend.ensureService(t, "placeholder", placeholder.Service)

	return backend
}

func (b *testSubscriptionBackend) Store() Store {
	return b.store
}

func (b *testSubscriptionBackend) ensurePeering(t *testing.T, name string) (uint64, string) {
	b.lastIdx++
	return b.lastIdx, setupTestPeering(t, b.store, name, b.lastIdx)
}

func (b *testSubscriptionBackend) ensureConfigEntry(t *testing.T, entry structs.ConfigEntry) uint64 {
	require.NoError(t, entry.Normalize())
	require.NoError(t, entry.Validate())

	b.lastIdx++
	require.NoError(t, b.store.EnsureConfigEntry(b.lastIdx, entry))
	return b.lastIdx
}

func (b *testSubscriptionBackend) ensureNode(t *testing.T, node *structs.Node) uint64 {
	b.lastIdx++
	require.NoError(t, b.store.EnsureNode(b.lastIdx, node))
	return b.lastIdx
}

func (b *testSubscriptionBackend) ensureService(t *testing.T, node string, svc *structs.NodeService) uint64 {
	b.lastIdx++
	require.NoError(t, b.store.EnsureService(b.lastIdx, node, svc))
	return b.lastIdx
}

func (b *testSubscriptionBackend) ensureCheck(t *testing.T, hc *structs.HealthCheck) uint64 {
	b.lastIdx++
	require.NoError(t, b.store.EnsureCheck(b.lastIdx, hc))
	return b.lastIdx
}

func (b *testSubscriptionBackend) deleteService(t *testing.T, nodeName, serviceID string) uint64 {
	b.lastIdx++
	require.NoError(t, b.store.DeleteService(b.lastIdx, nodeName, serviceID, nil, ""))
	return b.lastIdx
}

func setupTestPeering(t *testing.T, store *state.Store, name string, index uint64) string {
	err := store.PeeringWrite(index, &pbpeering.Peering{
		Name: name,
	})
	require.NoError(t, err)

	_, p, err := store.PeeringRead(nil, state.Query{Value: name})
	require.NoError(t, err)
	require.NotNil(t, p)

	return p.ID
}

func newStateStore(t *testing.T, publisher *stream.EventPublisher) *state.Store {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	gc, err := state.NewTombstoneGC(time.Second, time.Millisecond)
	require.NoError(t, err)

	store := state.NewStateStoreWithEventPublisher(gc, publisher)
	require.NoError(t, publisher.RegisterHandler(state.EventTopicServiceHealth, store.ServiceHealthSnapshot))
	require.NoError(t, publisher.RegisterHandler(state.EventTopicServiceHealthConnect, store.ServiceHealthSnapshot))
	go publisher.Run(ctx)

	return store
}

func expectEvents(
	t *testing.T,
	ch <-chan cache.UpdateEvent,
	checkFns ...func(t *testing.T, got cache.UpdateEvent),
) {
	t.Helper()

	num := len(checkFns)

	var out []cache.UpdateEvent

	if num == 0 {
		// No updates should be received.
		select {
		case <-ch:
			t.Fatalf("received unexpected update")
		case <-time.After(100 * time.Millisecond):
			// Expect this to fire
		}
		return
	}

	const timeout = 10 * time.Second
	timeoutCh := time.After(timeout)

	for len(out) < num {
		select {
		case <-timeoutCh:
			t.Fatalf("timed out with %d of %d events after %v", len(out), num, timeout)
		case evt := <-ch:
			out = append(out, evt)
		}
	}

	select {
	case <-time.After(100 * time.Millisecond):
	case evt := <-ch:
		t.Fatalf("expected only %d events but got more; prev %+v; next %+v;", num, out, evt)
	}

	require.Len(t, out, num)

	sort.SliceStable(out, func(i, j int) bool {
		return out[i].CorrelationID < out[j].CorrelationID
	})

	for i := 0; i < num; i++ {
		checkFns[i](t, out[i])
	}
}

func checkEvent(
	t *testing.T,
	got cache.UpdateEvent,
	correlationID string,
	expectNodes int,
	serviceKindPairs ...string) {
	t.Helper()

	require.True(t, len(serviceKindPairs) == 2*expectNodes, "sanity check")

	require.Equal(t, correlationID, got.CorrelationID)

	evt := got.Result.(*pbservice.IndexedCheckServiceNodes)
	require.Equal(t, uint64(0), evt.Index)

	if expectNodes == 0 {
		require.Len(t, evt.Nodes, 0)
	} else {
		require.Len(t, evt.Nodes, expectNodes)

		for i := 0; i < expectNodes; i++ {
			expectName := serviceKindPairs[i*2]
			expectKind := serviceKindPairs[i*2+1]
			require.Equal(t, expectName, evt.Nodes[i].Service.Service)
			require.Equal(t, expectKind, evt.Nodes[i].Service.Kind)
		}
	}
}
