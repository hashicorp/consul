package state

import (
	"testing"

	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/require"
)

func TestRegistrationEvents(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	s := testStateStore(t)

	nodeID := makeRandomNodeID(t)

	// Register a first node with a couple services on it.
	{
		req := &structs.RegisterRequest{
			ID:      nodeID,
			Node:    "node1",
			Address: "1.2.3.4",
			Service: &structs.NodeService{
				ID:      "api1",
				Service: "api",
				Address: "1.1.1.1",
				Port:    8080,
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:    "node1",
					CheckID: "check1",
					Name:    "node check",
					Status:  "passing",
				},
				&structs.HealthCheck{
					Node:      "node1",
					CheckID:   "check2",
					Name:      "api check",
					ServiceID: "api1",
				},
			},
		}
		require.NoError(s.EnsureRegistration(1, req))
	}
	{
		req := &structs.RegisterRequest{
			ID:      nodeID,
			Node:    "node1",
			Address: "1.2.3.4",
			Service: &structs.NodeService{
				ID:      "redis1",
				Service: "redis",
				Address: "1.1.1.1",
				Port:    8080,
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:      "node1",
					CheckID:   "check3",
					Name:      "redis check 1",
					ServiceID: "redis1",
				},
				&structs.HealthCheck{
					Node:      "node1",
					CheckID:   "check4",
					Name:      "redis check 2",
					ServiceID: "redis1",
				},
			},
		}
		require.NoError(s.EnsureRegistration(2, req))
	}

	expected := []stream.Event{
		stream.Event{
			Index: 2,
			Key:   "api",
			ServiceHealth: &stream.ServiceHealthUpdate{
				Node:    "node1",
				Id:      string(nodeID),
				Address: "1.2.3.4",
				Service: "api",
				Checks: []*stream.HealthCheck{
					{
						Name:    "node check",
						Status:  "passing",
						CheckID: "check1",
					},
					{
						Name:        "api check",
						Status:      "critical",
						CheckID:     "check2",
						ServiceID:   "api1",
						ServiceName: "api",
					},
				},
			},
		},
		stream.Event{
			Index: 2,
			Key:   "redis",
			ServiceHealth: &stream.ServiceHealthUpdate{
				Node:    "node1",
				Id:      string(nodeID),
				Address: "1.2.3.4",
				Service: "redis",
				Checks: []*stream.HealthCheck{
					{
						Name:    "node check",
						Status:  "passing",
						CheckID: "check1",
					},
					{
						Name:        "redis check 1",
						Status:      "critical",
						CheckID:     "check3",
						ServiceID:   "redis1",
						ServiceName: "redis",
					},
					{
						Name:        "redis check 2",
						Status:      "critical",
						CheckID:     "check4",
						ServiceID:   "redis1",
						ServiceName: "redis",
					},
				},
			},
		},
	}

	// Check the output for all the services on node1.
	{
		tx := s.db.Txn(false)
		events, err := s.RegistrationEvents(tx, "node1", "")
		require.NoError(err)
		require.Equal(expected, events)
		tx.Abort()
	}

	// Register a totally different node.
	node2ID := makeRandomNodeID(t)
	{
		req := &structs.RegisterRequest{
			ID:      node2ID,
			Node:    "node2",
			Address: "1.2.3.4",
			Service: &structs.NodeService{
				ID:      "database1",
				Service: "database",
				Address: "1.1.1.1",
				Port:    8080,
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:      "node2",
					CheckID:   "check1",
					Name:      "database check",
					ServiceID: "database1",
				},
			},
		}
		require.NoError(s.EnsureRegistration(3, req))
	}

	// Check the output node1 again.
	expected[0].Index = 3
	expected[1].Index = 3
	{
		tx := s.db.Txn(false)
		events, err := s.RegistrationEvents(tx, "node1", "")
		require.NoError(err)
		require.Equal(expected, events)
		tx.Abort()
	}
}
