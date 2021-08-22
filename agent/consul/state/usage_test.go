package state

import (
	"testing"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
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

	require.NoError(t, s.DeleteNode(2, "node2", nil))
	idx, usage, err = s.NodeUsage()
	require.NoError(t, err)
	require.Equal(t, idx, uint64(2))
	require.Equal(t, usage.Nodes, 1)
}

func TestStateStore_Usage_ServiceUsageEmpty(t *testing.T) {
	s := testStateStore(t)

	// No services have been registered, and thus no usage entry exists
	idx, usage, err := s.ServiceUsage()
	require.NoError(t, err)
	require.Equal(t, idx, uint64(0))
	require.Equal(t, usage.Services, 0)
	require.Equal(t, usage.ServiceInstances, 0)
}

func TestStateStore_Usage_ServiceUsage(t *testing.T) {
	s := testStateStore(t)

	testRegisterNode(t, s, 0, "node1")
	testRegisterNode(t, s, 1, "node2")
	testRegisterService(t, s, 8, "node1", "service1")
	testRegisterService(t, s, 9, "node2", "service1")
	testRegisterService(t, s, 10, "node2", "service2")

	idx, usage, err := s.ServiceUsage()
	require.NoError(t, err)
	require.Equal(t, idx, uint64(10))
	require.Equal(t, 2, usage.Services)
	require.Equal(t, 3, usage.ServiceInstances)
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
		Address: "1.1.1.1",
		Port:    1111,
	}

	// Register multiple instances on a single node to test that we do not
	// double count deletions within the same transaction.
	require.NoError(t, s.EnsureService(1, "node1", svc1))
	require.NoError(t, s.EnsureService(2, "node1", svc2))

	idx, usage, err := s.ServiceUsage()
	require.NoError(t, err)
	require.Equal(t, idx, uint64(2))
	require.Equal(t, usage.Services, 1)
	require.Equal(t, usage.ServiceInstances, 2)

	require.NoError(t, s.DeleteNode(3, "node1", nil))

	idx, usage, err = s.ServiceUsage()
	require.NoError(t, err)
	require.Equal(t, idx, uint64(3))
	require.Equal(t, usage.Services, 0)
	require.Equal(t, usage.ServiceInstances, 0)
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

	idx, usage, err := s.ServiceUsage()
	require.NoError(t, err)
	require.Equal(t, idx, uint64(9))
	require.Equal(t, usage.Services, 1)
	require.Equal(t, usage.ServiceInstances, 2)
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
		},
	}

	err := updateUsage(txn, changes)
	require.NoError(t, err)

	// Check that we do not underflow
	u, err := txn.First("usage", "id", "nodes")
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

func TestStateStore_Usage_ServiceUsage_updatingServiceName(t *testing.T) {
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
		idx, usage, err := s.ServiceUsage()
		require.NoError(t, err)
		require.Equal(t, idx, uint64(2))
		require.Equal(t, usage.Services, 1)
		require.Equal(t, usage.ServiceInstances, 1)
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
			ID:      "service3",
			Service: "before",
			Address: "1.1.1.3",
			Port:    1111,
		}
		require.NoError(t, s.EnsureService(4, "node1", svc3))

		idx, usage, err := s.ServiceUsage()
		require.NoError(t, err)
		require.Equal(t, idx, uint64(4))
		require.Equal(t, usage.Services, 2)
		require.Equal(t, usage.ServiceInstances, 3)

		update := &structs.NodeService{
			ID:      "service2",
			Service: "another-name",
			Address: "1.1.1.2",
			Port:    1111,
		}
		require.NoError(t, s.EnsureService(5, "node1", update))

		idx, usage, err = s.ServiceUsage()
		require.NoError(t, err)
		require.Equal(t, idx, uint64(5))
		require.Equal(t, usage.Services, 3)
		require.Equal(t, usage.ServiceInstances, 3)
	})
}
