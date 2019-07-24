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
			Payload: &stream.Event_ServiceHealth{
				ServiceHealth: &stream.ServiceHealthUpdate{
					Op: stream.CatalogOp_Register,
					ServiceNode: &stream.CheckServiceNode{
						Node: &stream.Node{
							Node:      "node1",
							ID:        nodeID,
							Address:   "1.2.3.4",
							RaftIndex: stream.RaftIndex{CreateIndex: 1, ModifyIndex: 1},
						},
						Service: &stream.NodeService{
							ID:        "api1",
							Service:   "api",
							Address:   "1.1.1.1",
							Port:      8080,
							RaftIndex: stream.RaftIndex{CreateIndex: 1, ModifyIndex: 1},
							Weights:   &stream.Weights{Passing: 1, Warning: 1},
						},
						Checks: []*stream.HealthCheck{
							{
								Name:      "node check",
								Node:      "node1",
								Status:    "passing",
								CheckID:   "check1",
								RaftIndex: stream.RaftIndex{CreateIndex: 1, ModifyIndex: 1},
							},
							{
								Name:        "api check",
								Node:        "node1",
								Status:      "critical",
								CheckID:     "check2",
								ServiceID:   "api1",
								ServiceName: "api",
								RaftIndex:   stream.RaftIndex{CreateIndex: 1, ModifyIndex: 1},
							},
						},
					},
				},
			},
		},
		stream.Event{
			Index: 2,
			Key:   "redis",
			Payload: &stream.Event_ServiceHealth{
				ServiceHealth: &stream.ServiceHealthUpdate{
					Op: stream.CatalogOp_Register,
					ServiceNode: &stream.CheckServiceNode{
						Node: &stream.Node{
							Node:      "node1",
							ID:        nodeID,
							Address:   "1.2.3.4",
							RaftIndex: stream.RaftIndex{CreateIndex: 1, ModifyIndex: 1},
						},
						Service: &stream.NodeService{
							ID:        "redis1",
							Service:   "redis",
							Address:   "1.1.1.1",
							Port:      8080,
							RaftIndex: stream.RaftIndex{CreateIndex: 2, ModifyIndex: 2},
							Weights:   &stream.Weights{Passing: 1, Warning: 1},
						},
						Checks: []*stream.HealthCheck{
							{
								Name:      "node check",
								Node:      "node1",
								Status:    "passing",
								CheckID:   "check1",
								RaftIndex: stream.RaftIndex{CreateIndex: 1, ModifyIndex: 1},
							},
							{
								Name:        "redis check 1",
								Node:        "node1",
								Status:      "critical",
								CheckID:     "check3",
								ServiceID:   "redis1",
								ServiceName: "redis",
								RaftIndex:   stream.RaftIndex{CreateIndex: 2, ModifyIndex: 2},
							},
							{
								Name:        "redis check 2",
								Node:        "node1",
								Status:      "critical",
								CheckID:     "check4",
								ServiceID:   "redis1",
								ServiceName: "redis",
								RaftIndex:   stream.RaftIndex{CreateIndex: 2, ModifyIndex: 2},
							},
						},
					},
				},
			},
		},
	}

	// Check the output for all the services on node1.
	{
		tx := s.db.Txn(false)
		events, err := s.RegistrationEvents(tx, 2, "node1", "")
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
		events, err := s.RegistrationEvents(tx, 3, "node1", "")
		require.NoError(err)
		require.Equal(expected, events)
		tx.Abort()
	}
}
