package state

import (
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	memdb "github.com/hashicorp/go-memdb"
	"github.com/stretchr/testify/require"
)

func TestStateStore_Usage_NodeCount(t *testing.T) {
	s := testStateStore(t)

	// No nodes have been registered, and thus no usage entry exists
	idx, count, err := s.NodeCount()
	require.NoError(t, err)
	require.Equal(t, idx, uint64(0))
	require.Equal(t, count, 0)

	testRegisterNode(t, s, 0, "node1")
	testRegisterNode(t, s, 1, "node2")

	idx, count, err = s.NodeCount()
	require.NoError(t, err)
	require.Equal(t, idx, uint64(1))
	require.Equal(t, count, 2)
}

func TestStateStore_Usage_NodeCount_Delete(t *testing.T) {
	s := testStateStore(t)

	testRegisterNode(t, s, 0, "node1")
	testRegisterNode(t, s, 1, "node2")

	idx, count, err := s.NodeCount()
	require.NoError(t, err)
	require.Equal(t, idx, uint64(1))
	require.Equal(t, count, 2)

	require.NoError(t, s.DeleteNode(2, "node2"))
	idx, count, err = s.NodeCount()
	require.NoError(t, err)
	require.Equal(t, idx, uint64(2))
	require.Equal(t, count, 1)
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
	require.NoError(t, restore.Commit())

	idx, count, err := s.NodeCount()
	require.NoError(t, err)
	require.Equal(t, idx, uint64(9))
	require.Equal(t, count, 1)
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
	require.Error(t, err)
	require.Contains(t, err.Error(), "negative count")

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
	require.Error(t, err)
	require.Contains(t, err.Error(), "negative count")
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
