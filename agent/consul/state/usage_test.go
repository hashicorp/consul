package state

import (
	"context"
	"testing"
	"time"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestStateStore_Usage_NodeUsage(t *testing.T) {
	s := testStateStore(t)

	// No nodes have been registered, and thus no usage entry exists
	idx, usage, err := s.NodeUsage()
	require.NoError(t, err)
	require.Equal(t, idx, uint64(0))
	require.Equal(t, usage.Nodes, 0)

	testRegisterNode(t, s, 0, "node1")
	testRegisterNode(t, s, 1, "node2")

	idx, usage, err = s.NodeUsage()
	require.NoError(t, err)
	require.Equal(t, idx, uint64(1))
	require.Equal(t, usage.Nodes, 2)
}

func TestStateStore_Usage_NodeUsage_Delete(t *testing.T) {
	s := testStateStore(t)

	testRegisterNode(t, s, 0, "node1")
	testRegisterNode(t, s, 1, "node2")

	idx, usage, err := s.NodeUsage()
	require.NoError(t, err)
	require.Equal(t, idx, uint64(1))
	require.Equal(t, usage.Nodes, 2)

	require.NoError(t, s.DeleteNode(2, "node2", nil, ""))
	idx, usage, err = s.NodeUsage()
	require.NoError(t, err)
	require.Equal(t, idx, uint64(2))
	require.Equal(t, usage.Nodes, 1)
}

// Test that nodes added/deleted from peers are not counted.
func TestStateStore_Usage_NodeUsagePeering(t *testing.T) {
	s := testStateStore(t)

	// Register two nodes, one local, one from a remote peer.
	testRegisterNodePeer(t, s, 0, "node1", "peer")
	testRegisterNode(t, s, 1, "node2")

	testutil.RunStep(t, "write node", func(t *testing.T) {
		// Test that we're only tracking the local node.
		idx, usage, err := s.NodeUsage()
		require.NoError(t, err)
		require.Equal(t, uint64(1), idx)
		require.Equal(t, 1, usage.Nodes)
	})

	testutil.RunStep(t, "delete node", func(t *testing.T) {
		// Deregister the remote peered node. Count should remain the same.
		require.NoError(t, s.DeleteNode(2, "node1", nil, "peer"))
		idx, usage, err := s.NodeUsage()
		require.NoError(t, err)
		require.Equal(t, uint64(1), idx)
		require.Equal(t, 1, usage.Nodes)
	})
}

func TestStateStore_Usage_PeeringUsage(t *testing.T) {
	s := testStateStore(t)

	// No nodes have been registered, and thus no usage entry exists
	idx, usage, err := s.PeeringUsage()
	require.NoError(t, err)
	require.Equal(t, idx, uint64(0))
	require.Equal(t, usage.Peerings, 0)

	testRegisterPeering(t, s, 0, "test-peering1")
	testRegisterPeering(t, s, 1, "test-peering2")

	idx, usage, err = s.PeeringUsage()
	require.NoError(t, err)
	require.Equal(t, idx, uint64(1))
	require.Equal(t, usage.Peerings, 2)
}

func TestStateStore_Usage_PeeringUsage_Delete(t *testing.T) {
	s := testStateStore(t)

	testRegisterPeering(t, s, 0, "test-peering1")
	peering2 := testRegisterPeering(t, s, 1, "test-peering2")

	idx, usage, err := s.PeeringUsage()
	require.NoError(t, err)
	require.Equal(t, idx, uint64(1))
	require.Equal(t, usage.Peerings, 2)

	require.NoError(t, s.PeeringTerminateByID(2, peering2.ID))
	idx, usage, err = s.PeeringUsage()
	require.NoError(t, err)
	require.Equal(t, uint64(2), idx)
	require.Equal(t, 2, usage.Peerings)
}

func TestStateStore_Usage_KVUsage(t *testing.T) {
	s := testStateStore(t)

	// No keys have been registered, and thus no usage entry exists
	idx, usage, err := s.KVUsage()
	require.NoError(t, err)
	require.Equal(t, idx, uint64(0))
	require.Equal(t, usage.KVCount, 0)

	testSetKey(t, s, 0, "key-1", "0", nil)
	testSetKey(t, s, 1, "key-2", "0", nil)
	testSetKey(t, s, 2, "key-2", "1", nil)

	idx, usage, err = s.KVUsage()
	require.NoError(t, err)
	require.Equal(t, idx, uint64(2))
	require.Equal(t, usage.KVCount, 2)
}

func TestStateStore_Usage_KVUsage_Delete(t *testing.T) {
	s := testStateStore(t)

	testSetKey(t, s, 0, "key-1", "0", nil)
	testSetKey(t, s, 1, "key-2", "0", nil)
	testSetKey(t, s, 2, "key-2", "1", nil)

	idx, usage, err := s.KVUsage()
	require.NoError(t, err)
	require.Equal(t, idx, uint64(2))
	require.Equal(t, usage.KVCount, 2)

	require.NoError(t, s.KVSDelete(3, "key-2", nil))
	idx, usage, err = s.KVUsage()
	require.NoError(t, err)
	require.Equal(t, idx, uint64(3))
	require.Equal(t, usage.KVCount, 1)
}

func TestStateStore_Usage_ServiceUsageEmpty(t *testing.T) {
	s := testStateStore(t)

	// No services have been registered, and thus no usage entry exists
	idx, usage, err := s.ServiceUsage(nil)
	require.NoError(t, err)
	require.Equal(t, idx, uint64(0))
	require.Equal(t, usage.Services, 0)
	require.Equal(t, usage.ServiceInstances, 0)
	for k := range usage.ConnectServiceInstances {
		require.Equal(t, 0, usage.ConnectServiceInstances[k])
	}
	require.Equal(t, 0, usage.BillableServiceInstances)
}

func TestStateStore_Usage_ServiceUsage(t *testing.T) {
	s := testStateStore(t)

	testRegisterNode(t, s, 0, "node1")
	testRegisterNode(t, s, 1, "node2")
	testRegisterService(t, s, 8, "node1", "service1")
	testRegisterService(t, s, 9, "node2", "service1")
	testRegisterService(t, s, 10, "node2", "service2")
	testRegisterSidecarProxy(t, s, 11, "node1", "service1")
	testRegisterSidecarProxy(t, s, 12, "node2", "service1")
	testRegisterConnectNativeService(t, s, 13, "node1", "service-native")
	testRegisterConnectNativeService(t, s, 14, "node2", "service-native")
	testRegisterConnectNativeService(t, s, 15, "node2", "service-native-1")

	ws := memdb.NewWatchSet()
	idx, usage, err := s.ServiceUsage(ws)
	require.NoError(t, err)
	require.Equal(t, idx, uint64(15))
	require.Equal(t, 5, usage.Services)
	require.Equal(t, 8, usage.ServiceInstances)
	require.Equal(t, 2, usage.ConnectServiceInstances[string(structs.ServiceKindConnectProxy)])
	require.Equal(t, 3, usage.ConnectServiceInstances[connectNativeInstancesTable])
	require.Equal(t, 6, usage.BillableServiceInstances)

	testRegisterSidecarProxy(t, s, 16, "node2", "service2")

	select {
	case <-ws.WatchCh(context.Background()):
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting on WatchSet")
	}
}

func TestStateStore_Usage_ServiceUsage_DeleteNode(t *testing.T) {
	s := testStateStore(t)
	testRegisterNode(t, s, 1, "node1")

	svc1 := &structs.NodeService{
		ID:      "service1",
		Service: "test",
		Address: "1.1.1.1",
		Port:    1111,
	}
	svc2 := &structs.NodeService{
		ID:      "service2",
		Service: "test",
		Tags:    []string{},
		Address: "1.1.1.1",
	}

	// Register multiple instances on a single node to test that we do not
	// double count deletions within the same transaction.
	require.NoError(t, s.EnsureService(1, "node1", svc1))
	require.NoError(t, s.EnsureService(2, "node1", svc2))
	testRegisterSidecarProxy(t, s, 3, "node1", "service2")
	testRegisterConnectNativeService(t, s, 4, "node1", "service-connect")

	idx, usage, err := s.ServiceUsage(nil)
	require.NoError(t, err)
	require.Equal(t, idx, uint64(4))
	require.Equal(t, 3, usage.Services)
	require.Equal(t, 4, usage.ServiceInstances)
	require.Equal(t, 1, usage.ConnectServiceInstances[string(structs.ServiceKindConnectProxy)])
	require.Equal(t, 1, usage.ConnectServiceInstances[connectNativeInstancesTable])
	require.Equal(t, 3, usage.BillableServiceInstances)

	require.NoError(t, s.DeleteNode(4, "node1", nil, ""))

	idx, usage, err = s.ServiceUsage(nil)
	require.NoError(t, err)
	require.Equal(t, idx, uint64(4))
	require.Equal(t, usage.Services, 0)
	require.Equal(t, usage.ServiceInstances, 0)
	for k := range usage.ConnectServiceInstances {
		require.Equal(t, 0, usage.ConnectServiceInstances[k])
	}
	require.Equal(t, 0, usage.BillableServiceInstances)
}

// Test that services from remote peers aren't counted in writes or deletes.
func TestStateStore_Usage_ServiceUsagePeering(t *testing.T) {
	s := testStateStore(t)
	peerName := "peer"

	// Register remote peer node/services.
	testRegisterNodePeer(t, s, 0, "node1", peerName)
	testRegisterServicePeer(t, s, 1, "node1", "service1", peerName)
	testRegisterSidecarProxyPeer(t, s, 2, "node1", "service1", peerName)
	testRegisterConnectNativeServicePeer(t, s, 3, "node1", "service-native", peerName)

	// Register local node/services.
	testRegisterNode(t, s, 4, "node2")
	testRegisterService(t, s, 5, "node2", "service1")
	testRegisterSidecarProxy(t, s, 6, "node2", "service1")
	testRegisterConnectNativeService(t, s, 7, "node2", "service-native")

	testutil.RunStep(t, "writes", func(t *testing.T) {
		idx, usage, err := s.ServiceUsage(nil)
		require.NoError(t, err)
		require.Equal(t, uint64(7), idx)
		require.Equal(t, 3, usage.Services)
		require.Equal(t, 3, usage.ServiceInstances)
		require.Equal(t, 1, usage.ConnectServiceInstances[string(structs.ServiceKindConnectProxy)])
		require.Equal(t, 1, usage.ConnectServiceInstances[connectNativeInstancesTable])
		require.Equal(t, 2, usage.BillableServiceInstances)
	})

	testutil.RunStep(t, "deletes", func(t *testing.T) {
		require.NoError(t, s.DeleteNode(7, "node1", nil, peerName))
		require.NoError(t, s.DeleteNode(8, "node2", nil, ""))
		idx, usage, err := s.ServiceUsage(nil)
		require.NoError(t, err)
		require.Equal(t, uint64(8), idx)
		require.Equal(t, 0, usage.Services)
		require.Equal(t, 0, usage.ServiceInstances)
		require.Equal(t, 0, usage.ConnectServiceInstances[string(structs.ServiceKindConnectProxy)])
		require.Equal(t, 0, usage.ConnectServiceInstances[connectNativeInstancesTable])
		require.Equal(t, 0, usage.BillableServiceInstances)
	})
}

func TestStateStore_Usage_Restore(t *testing.T) {
	s := testStateStore(t)
	restore := s.Restore()
	restore.Registration(9, &structs.RegisterRequest{
		Node: "test-node",
		Service: &structs.NodeService{
			ID:      "mysql",
			Service: "mysql",
			Port:    8080,
			Address: "198.18.0.2",
		},
	})
	restore.Registration(9, &structs.RegisterRequest{
		Node: "test-node",
		Service: &structs.NodeService{
			ID:      "mysql1",
			Service: "mysql",
			Port:    8081,
			Address: "198.18.0.2",
		},
	})
	require.NoError(t, restore.Commit())

	idx, nodeUsage, err := s.NodeUsage()
	require.NoError(t, err)
	require.Equal(t, idx, uint64(9))
	require.Equal(t, nodeUsage.Nodes, 1)

	idx, usage, err := s.ServiceUsage(nil)
	require.NoError(t, err)
	require.Equal(t, idx, uint64(9))
	require.Equal(t, usage.Services, 1)
	require.Equal(t, usage.ServiceInstances, 2)
	require.Equal(t, usage.BillableServiceInstances, 2)
}

func TestStateStore_Usage_updateUsage_Underflow(t *testing.T) {
	s := testStateStore(t)
	txn := s.db.WriteTxn(1)

	// A single delete change will cause a negative count
	changes := Changes{
		Index: 1,
		Changes: memdb.Changes{
			{
				Table:  "nodes",
				Before: &structs.Node{},
				After:  nil,
			},
			{
				Table: tableServices,
				Before: &structs.ServiceNode{
					ID:          "service-1-connect-proxy",
					ServiceKind: structs.ServiceKindConnectProxy,
					ServiceID:   "service-1",
					ServiceName: "service",
				},
			},
		},
	}

	err := updateUsage(txn, changes)
	require.NoError(t, err)

	// Check that we do not underflow
	u, err := txn.First("usage", "id", "nodes")
	require.NoError(t, err)
	require.Equal(t, 0, u.(*UsageEntry).Count)

	u, err = txn.First("usage", "id", connectUsageTableName(string(structs.ServiceKindConnectProxy)))
	require.NoError(t, err)
	require.Equal(t, 0, u.(*UsageEntry).Count)

	// A insert a change to create a usage entry
	changes = Changes{
		Index: 1,
		Changes: memdb.Changes{
			{
				Table:  "nodes",
				Before: nil,
				After:  &structs.Node{},
			},
		},
	}

	err = updateUsage(txn, changes)
	require.NoError(t, err)

	// Two deletes will cause a negative count now
	changes = Changes{
		Index: 1,
		Changes: memdb.Changes{
			{
				Table:  "nodes",
				Before: &structs.Node{},
				After:  nil,
			},
			{
				Table:  "nodes",
				Before: &structs.Node{},
				After:  nil,
			},
		},
	}

	err = updateUsage(txn, changes)
	require.NoError(t, err)

	// Check that we do not underflow
	u, err = txn.First("usage", "id", "nodes")
	require.NoError(t, err)
	require.Equal(t, 0, u.(*UsageEntry).Count)
}

func TestStateStore_Usage_ServiceUsage_updatingService(t *testing.T) {
	s := testStateStore(t)
	testRegisterNode(t, s, 1, "node1")
	testRegisterService(t, s, 1, "node1", "service1")

	t.Run("rename service with a single instance", func(t *testing.T) {
		svc := &structs.NodeService{
			ID:      "service1",
			Service: "after",
			Address: "1.1.1.1",
			Port:    1111,
		}
		require.NoError(t, s.EnsureService(2, "node1", svc))

		// We renamed a service with a single instance, so we maintain 1 service.
		idx, usage, err := s.ServiceUsage(nil)
		require.NoError(t, err)
		require.Equal(t, idx, uint64(2))
		require.Equal(t, usage.Services, 1)
		require.Equal(t, usage.ServiceInstances, 1)
		require.Equal(t, usage.BillableServiceInstances, 1)
	})

	t.Run("update service to be connect native", func(t *testing.T) {
		svc := &structs.NodeService{
			ID:      "service1",
			Service: "after",
			Address: "1.1.1.1",
			Port:    1111,
			Connect: structs.ServiceConnect{
				Native: true,
			},
		}
		require.NoError(t, s.EnsureService(3, "node1", svc))

		// We renamed a service with a single instance, so we maintain 1 service.
		idx, usage, err := s.ServiceUsage(nil)
		require.NoError(t, err)
		require.Equal(t, idx, uint64(3))
		require.Equal(t, usage.Services, 1)
		require.Equal(t, usage.ServiceInstances, 1)
		require.Equal(t, 1, usage.ConnectServiceInstances[connectNativeInstancesTable])
		require.Equal(t, 1, usage.BillableServiceInstances)
	})

	t.Run("update service to not be connect native", func(t *testing.T) {
		svc := &structs.NodeService{
			ID:      "service1",
			Service: "after",
			Address: "1.1.1.1",
			Port:    1111,
			Connect: structs.ServiceConnect{
				Native: false,
			},
		}
		require.NoError(t, s.EnsureService(4, "node1", svc))

		// We renamed a service with a single instance, so we maintain 1 service.
		idx, usage, err := s.ServiceUsage(nil)
		require.NoError(t, err)
		require.Equal(t, idx, uint64(4))
		require.Equal(t, usage.Services, 1)
		require.Equal(t, usage.ServiceInstances, 1)
		require.Equal(t, 0, usage.ConnectServiceInstances[connectNativeInstancesTable])
		require.Equal(t, 1, usage.BillableServiceInstances)
	})

	t.Run("rename service with a multiple instances", func(t *testing.T) {
		svc2 := &structs.NodeService{
			ID:      "service2",
			Service: "before",
			Address: "1.1.1.2",
			Port:    1111,
			Connect: structs.ServiceConnect{
				Native: true,
			},
		}
		require.NoError(t, s.EnsureService(5, "node1", svc2))

		svc3 := &structs.NodeService{
			ID:      "service3",
			Service: "before",
			Address: "1.1.1.3",
			Port:    1111,
			Connect: structs.ServiceConnect{
				Native: true,
			},
		}
		require.NoError(t, s.EnsureService(6, "node1", svc3))

		idx, usage, err := s.ServiceUsage(nil)
		require.NoError(t, err)
		require.Equal(t, idx, uint64(6))
		require.Equal(t, usage.Services, 2)
		require.Equal(t, usage.ServiceInstances, 3)
		require.Equal(t, 2, usage.ConnectServiceInstances[connectNativeInstancesTable])
		require.Equal(t, 3, usage.BillableServiceInstances)

		update := &structs.NodeService{
			ID:      "service2",
			Service: "another-name",
			Address: "1.1.1.2",
			Port:    1111,
			Connect: structs.ServiceConnect{
				Native: true,
			},
		}
		require.NoError(t, s.EnsureService(7, "node1", update))

		idx, usage, err = s.ServiceUsage(nil)
		require.NoError(t, err)
		require.Equal(t, idx, uint64(7))
		require.Equal(t, usage.Services, 3)
		require.Equal(t, usage.ServiceInstances, 3)
		require.Equal(t, 2, usage.ConnectServiceInstances[connectNativeInstancesTable])
		require.Equal(t, 3, usage.BillableServiceInstances)

	})
}

func TestStateStore_Usage_ServiceUsage_updatingConnectProxy(t *testing.T) {
	s := testStateStore(t)
	testRegisterNode(t, s, 1, "node1")
	testRegisterService(t, s, 1, "node1", "service1")

	t.Run("change service to ConnectProxy", func(t *testing.T) {
		svc := &structs.NodeService{
			Kind:    structs.ServiceKindConnectProxy,
			ID:      "service1",
			Service: "after",
			Address: "1.1.1.1",
			Port:    1111,
		}
		require.NoError(t, s.EnsureService(2, "node1", svc))

		// We renamed a service with a single instance, so we maintain 1 service.
		idx, usage, err := s.ServiceUsage(nil)
		require.NoError(t, err)
		require.Equal(t, idx, uint64(2))
		require.Equal(t, usage.Services, 1)
		require.Equal(t, usage.ServiceInstances, 1)
		require.Equal(t, 1, usage.ConnectServiceInstances[string(structs.ServiceKindConnectProxy)])
		require.Equal(t, 0, usage.BillableServiceInstances)
	})

	t.Run("rename service with a multiple instances", func(t *testing.T) {
		svc2 := &structs.NodeService{
			ID:      "service2",
			Service: "before",
			Address: "1.1.1.2",
			Port:    1111,
		}
		require.NoError(t, s.EnsureService(3, "node1", svc2))

		svc3 := &structs.NodeService{
			Kind:    structs.ServiceKindConnectProxy,
			ID:      "service3",
			Service: "before",
			Address: "1.1.1.3",
			Port:    1111,
		}
		require.NoError(t, s.EnsureService(4, "node1", svc3))

		idx, usage, err := s.ServiceUsage(nil)
		require.NoError(t, err)
		require.Equal(t, idx, uint64(4))
		require.Equal(t, usage.Services, 2)
		require.Equal(t, usage.ServiceInstances, 3)
		require.Equal(t, 2, usage.ConnectServiceInstances[string(structs.ServiceKindConnectProxy)])
		require.Equal(t, 1, usage.BillableServiceInstances)

		update := &structs.NodeService{
			ID:      "service3",
			Service: "another-name",
			Address: "1.1.1.2",
			Port:    1111,
		}
		require.NoError(t, s.EnsureService(5, "node1", update))

		idx, usage, err = s.ServiceUsage(nil)
		require.NoError(t, err)
		require.Equal(t, idx, uint64(5))
		require.Equal(t, usage.Services, 3)
		require.Equal(t, usage.ServiceInstances, 3)
		require.Equal(t, 1, usage.ConnectServiceInstances[string(structs.ServiceKindConnectProxy)])
		require.Equal(t, 2, usage.BillableServiceInstances)
	})
}

func TestStateStore_Usage_ConfigEntries(t *testing.T) {
	s := testStateStore(t)

	t.Run("empty store", func(t *testing.T) {
		i, usage, err := s.ConfigEntryUsage()
		require.NoError(t, err)
		require.Equal(t, uint64(0), i)
		for _, kind := range structs.AllConfigEntryKinds {
			require.Equal(t, 0, usage.ConfigByKind[kind])
		}
	})
	t.Run("with config entries", func(t *testing.T) {
		require.NoError(t, s.EnsureConfigEntry(1, &structs.ServiceConfigEntry{
			Kind:     structs.ServiceDefaults,
			Name:     "web",
			Protocol: "http",
		}))
		require.NoError(t, s.EnsureConfigEntry(2, &structs.ServiceResolverConfigEntry{
			Kind:          structs.ServiceResolver,
			Name:          "web",
			DefaultSubset: "v1",
			Subsets: map[string]structs.ServiceResolverSubset{
				"v1": {
					Filter: "Service.Meta.version == v1",
				},
				"v2": {
					Filter: "Service.Meta.version == v2",
				},
			},
		}))
		require.NoError(t, s.EnsureConfigEntry(3, &structs.ServiceIntentionsConfigEntry{
			Kind: structs.ServiceIntentions,
			Name: "web",
		}))

		i, usage, err := s.ConfigEntryUsage()
		require.NoError(t, err)
		require.Equal(t, uint64(3), i)
		require.Equal(t, 1, usage.ConfigByKind[structs.ServiceDefaults])
		require.Equal(t, 1, usage.ConfigByKind[structs.ServiceResolver])
		require.Equal(t, 1, usage.ConfigByKind[structs.ServiceIntentions])
	})
}
