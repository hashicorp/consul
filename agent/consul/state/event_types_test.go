package state

import (
	"fmt"
	"testing"

	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/types"
	"github.com/pascaldekloe/goe/verify"
	"github.com/stretchr/testify/require"
)

func TestRegistrationEvents_ServiceHealth(t *testing.T) {
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
					CheckServiceNode: &stream.CheckServiceNode{
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
					CheckServiceNode: &stream.CheckServiceNode{
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

func TestRegistrationEvents_ServiceHealthConnect(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	s := testStateStore(t)

	nodeID := makeRandomNodeID(t)

	// Register a first node with a native connect service, a non-native service,
	// and a proxy for the non-native service.
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
				Connect: structs.ServiceConnect{
					Native: true,
				},
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:    "node1",
					CheckID: "check1",
					Name:    "node check",
					Status:  "passing",
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
				Port:    7777,
			},
		}
		require.NoError(s.EnsureRegistration(2, req))
	}
	{
		req := &structs.RegisterRequest{
			ID:      nodeID,
			Node:    "node1",
			Address: "1.2.3.4",
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindConnectProxy,
				ID:      "redis1-proxy",
				Service: "redis-proxy",
				Address: "2.2.2.2",
				Port:    9999,
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "redis",
				},
			},
		}
		require.NoError(s.EnsureRegistration(3, req))
	}

	expected := []stream.Event{
		stream.Event{
			Topic: stream.Topic_ServiceHealthConnect,
			Index: 2,
			Key:   "api",
			Payload: &stream.Event_ServiceHealth{
				ServiceHealth: &stream.ServiceHealthUpdate{
					Op: stream.CatalogOp_Register,
					CheckServiceNode: &stream.CheckServiceNode{
						Node: &stream.Node{
							Node:      "node1",
							ID:        nodeID,
							Address:   "1.2.3.4",
							RaftIndex: stream.RaftIndex{CreateIndex: 1, ModifyIndex: 1},
						},
						Service: &stream.NodeService{
							ID:      "api1",
							Service: "api",
							Address: "1.1.1.1",
							Port:    8080,
							Connect: stream.ServiceConnect{
								Native: true,
							},
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
						},
					},
				},
			},
		},
		stream.Event{
			Topic: stream.Topic_ServiceHealthConnect,
			Index: 2,
			Key:   "redis",
			Payload: &stream.Event_ServiceHealth{
				ServiceHealth: &stream.ServiceHealthUpdate{
					Op: stream.CatalogOp_Register,
					CheckServiceNode: &stream.CheckServiceNode{
						Node: &stream.Node{
							Node:      "node1",
							ID:        nodeID,
							Address:   "1.2.3.4",
							RaftIndex: stream.RaftIndex{CreateIndex: 1, ModifyIndex: 1},
						},
						Service: &stream.NodeService{
							Kind:    structs.ServiceKindConnectProxy,
							ID:      "redis1-proxy",
							Service: "redis-proxy",
							Address: "2.2.2.2",
							Port:    9999,
							Proxy: stream.ConnectProxyConfig{
								DestinationServiceName: "redis",
							},
							RaftIndex: stream.RaftIndex{CreateIndex: 3, ModifyIndex: 3},
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
						},
					},
				},
			},
		},
	}

	// Check the output and make sure we only get events for the native service
	// and connect.
	{
		tx := s.db.Txn(false)
		events, err := s.RegistrationEvents(tx, 2, "node1", "")
		require.NoError(err)

		// Filter out only the events with the connect topic.
		var connectEvents []stream.Event
		for _, event := range events {
			if event.Topic == stream.Topic_ServiceHealthConnect {
				connectEvents = append(connectEvents, event)
			}
		}
		require.Equal(expected, connectEvents)
		tx.Abort()
	}
}

func TestTxnEvents_ServiceHealth(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	s := testStateStore(t)

	// Create some nodes.
	var nodes [5]structs.Node
	for i := 0; i < len(nodes); i++ {
		nodeName := fmt.Sprintf("node%d", i+1)
		nodes[i] = structs.Node{
			Node: nodeName,
			ID:   types.NodeID(testUUID()),
		}

		// Leave node5 to be created by an operation.
		idx := (i * 3) + 1
		if i < 5 {
			s.EnsureNode(uint64(idx), &nodes[i])
		}

		// Create a service.
		testRegisterService(t, s, uint64(idx+1), nodeName, fmt.Sprintf("svc%d", i+1))

		// Create a check.
		testRegisterCheck(t, s, uint64(idx+2), nodeName, "", types.CheckID(fmt.Sprintf("check%d", i+1)), "failing")
	}

	// Set up a transaction that hits every operation.
	ops := structs.TxnOps{
		&structs.TxnOp{
			Node: &structs.TxnNodeOp{
				Verb: api.NodeGet,
				Node: nodes[0],
			},
		},
		&structs.TxnOp{
			Node: &structs.TxnNodeOp{
				Verb: api.NodeSet,
				Node: nodes[4],
			},
		},
		&structs.TxnOp{
			Node: &structs.TxnNodeOp{
				Verb: api.NodeCAS,
				Node: structs.Node{
					Node:       "node2",
					ID:         nodes[1].ID,
					Datacenter: "dc2",
					RaftIndex:  structs.RaftIndex{ModifyIndex: 2},
				},
			},
		},
		&structs.TxnOp{
			Node: &structs.TxnNodeOp{
				Verb: api.NodeDelete,
				Node: structs.Node{Node: "node3"},
			},
		},
		&structs.TxnOp{
			Node: &structs.TxnNodeOp{
				Verb: api.NodeDeleteCAS,
				Node: structs.Node{
					Node:      "node4",
					RaftIndex: structs.RaftIndex{ModifyIndex: 4},
				},
			},
		},
		&structs.TxnOp{
			Service: &structs.TxnServiceOp{
				Verb:    api.ServiceGet,
				Node:    "node1",
				Service: structs.NodeService{ID: "svc1"},
			},
		},
		&structs.TxnOp{
			Service: &structs.TxnServiceOp{
				Verb:    api.ServiceSet,
				Node:    "node1",
				Service: structs.NodeService{ID: "svc5"},
			},
		},
		&structs.TxnOp{
			Service: &structs.TxnServiceOp{
				Verb: api.ServiceCAS,
				Node: "node1",
				Service: structs.NodeService{
					ID:        "svc2",
					Tags:      []string{"modified"},
					RaftIndex: structs.RaftIndex{ModifyIndex: 3},
				},
			},
		},
		&structs.TxnOp{
			Service: &structs.TxnServiceOp{
				Verb:    api.ServiceDelete,
				Node:    "node1",
				Service: structs.NodeService{ID: "svc3"},
			},
		},
		&structs.TxnOp{
			Service: &structs.TxnServiceOp{
				Verb: api.ServiceDeleteCAS,
				Node: "node1",
				Service: structs.NodeService{
					ID:        "svc4",
					RaftIndex: structs.RaftIndex{ModifyIndex: 5},
				},
			},
		},
		&structs.TxnOp{
			Check: &structs.TxnCheckOp{
				Verb:  api.CheckGet,
				Check: structs.HealthCheck{Node: "node1", CheckID: "check1"},
			},
		},
		&structs.TxnOp{
			Check: &structs.TxnCheckOp{
				Verb:  api.CheckSet,
				Check: structs.HealthCheck{Node: "node1", CheckID: "check5", Status: "passing"},
			},
		},
		&structs.TxnOp{
			Check: &structs.TxnCheckOp{
				Verb: api.CheckCAS,
				Check: structs.HealthCheck{
					Node:      "node1",
					CheckID:   "check2",
					Status:    "warning",
					RaftIndex: structs.RaftIndex{ModifyIndex: 3},
				},
			},
		},
		&structs.TxnOp{
			Check: &structs.TxnCheckOp{
				Verb:  api.CheckDelete,
				Check: structs.HealthCheck{Node: "node1", CheckID: "check3"},
			},
		},
		&structs.TxnOp{
			Check: &structs.TxnCheckOp{
				Verb: api.CheckDeleteCAS,
				Check: structs.HealthCheck{
					Node:      "node1",
					CheckID:   "check4",
					RaftIndex: structs.RaftIndex{ModifyIndex: 5},
				},
			},
		},
	}

	// Check the output for all the services on node1.
	expected := []stream.Event{
		stream.Event{
			Index: 2,
			Key:   "svc1",
			Payload: &stream.Event_ServiceHealth{
				ServiceHealth: &stream.ServiceHealthUpdate{
					Op: stream.CatalogOp_Register,
					CheckServiceNode: &stream.CheckServiceNode{
						Node: &stream.Node{
							Node:      "node1",
							ID:        nodes[0].ID,
							RaftIndex: stream.RaftIndex{CreateIndex: 1, ModifyIndex: 1},
						},
						Service: &stream.NodeService{
							ID:        "svc1",
							Service:   "svc1",
							Address:   "1.1.1.1",
							Port:      1111,
							Weights:   &stream.Weights{Passing: 1, Warning: 1},
							RaftIndex: stream.RaftIndex{CreateIndex: 2, ModifyIndex: 2},
						},
						Checks: []*stream.HealthCheck{
							{
								Node:      "node1",
								Status:    "failing",
								CheckID:   "check1",
								RaftIndex: stream.RaftIndex{CreateIndex: 3, ModifyIndex: 3},
							},
						},
					},
				},
			},
		},
		stream.Event{
			Index: 2,
			Key:   "svc2",
			Payload: &stream.Event_ServiceHealth{
				ServiceHealth: &stream.ServiceHealthUpdate{
					Op: stream.CatalogOp_Register,
					CheckServiceNode: &stream.CheckServiceNode{
						Node: &stream.Node{
							Node:      "node2",
							ID:        nodes[1].ID,
							RaftIndex: stream.RaftIndex{CreateIndex: 4, ModifyIndex: 4},
						},
						Service: &stream.NodeService{
							ID:        "svc2",
							Service:   "svc2",
							Address:   "1.1.1.1",
							Port:      1111,
							Weights:   &stream.Weights{Passing: 1, Warning: 1},
							RaftIndex: stream.RaftIndex{CreateIndex: 5, ModifyIndex: 5},
						},
						Checks: []*stream.HealthCheck{
							{
								Node:      "node2",
								Status:    "failing",
								CheckID:   "check2",
								RaftIndex: stream.RaftIndex{CreateIndex: 6, ModifyIndex: 6},
							},
						},
					},
				},
			},
		},
		stream.Event{
			Index: 2,
			Payload: &stream.Event_ServiceHealth{
				ServiceHealth: &stream.ServiceHealthUpdate{
					Op: stream.CatalogOp_Deregister,
					CheckServiceNode: &stream.CheckServiceNode{
						Node: &stream.Node{
							Node: "node3",
						},
					},
				},
			},
		},
		stream.Event{
			Index: 2,
			Payload: &stream.Event_ServiceHealth{
				ServiceHealth: &stream.ServiceHealthUpdate{
					Op: stream.CatalogOp_Deregister,
					CheckServiceNode: &stream.CheckServiceNode{
						Node: &stream.Node{
							Node: "node4",
						},
					},
				},
			},
		},
		stream.Event{
			Index: 2,
			Key:   "svc5",
			Payload: &stream.Event_ServiceHealth{
				ServiceHealth: &stream.ServiceHealthUpdate{
					Op: stream.CatalogOp_Register,
					CheckServiceNode: &stream.CheckServiceNode{
						Node: &stream.Node{
							Node:      "node5",
							ID:        nodes[4].ID,
							RaftIndex: stream.RaftIndex{CreateIndex: 13, ModifyIndex: 13},
						},
						Service: &stream.NodeService{
							ID:        "svc5",
							Service:   "svc5",
							Address:   "1.1.1.1",
							Port:      1111,
							Weights:   &stream.Weights{Passing: 1, Warning: 1},
							RaftIndex: stream.RaftIndex{CreateIndex: 14, ModifyIndex: 14},
						},
						Checks: []*stream.HealthCheck{
							{
								Node:      "node5",
								Status:    "failing",
								CheckID:   "check5",
								RaftIndex: stream.RaftIndex{CreateIndex: 15, ModifyIndex: 15},
							},
						},
					},
				},
			},
		},
	}
	{
		tx := s.db.Txn(false)
		events, err := s.TxnEvents(tx, 2, ops)
		require.NoError(err)
		verify.Values(t, "", events, expected)
		tx.Abort()
	}
}

func TestTxnEvents_ServiceHealthConnect(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	s := testStateStore(t)

	// Register a first node with a native connect service, a non-native service,
	// and a proxy for the non-native service.
	nodeID := types.NodeID(testUUID())
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
				Connect: structs.ServiceConnect{
					Native: true,
				},
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:    "node1",
					CheckID: "check1",
					Name:    "node check",
					Status:  "passing",
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
				Port:    7777,
			},
		}
		require.NoError(s.EnsureRegistration(2, req))
	}
	{
		req := &structs.RegisterRequest{
			ID:      nodeID,
			Node:    "node1",
			Address: "1.2.3.4",
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindConnectProxy,
				ID:      "redis1-proxy",
				Service: "redis-proxy",
				Address: "2.2.2.2",
				Port:    9999,
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "redis",
				},
			},
		}
		require.NoError(s.EnsureRegistration(3, req))
	}

	// Set up some txn ops that hit each service so we can be sure the connect ones
	// generate events for the connect topic and the others don't.
	ops := structs.TxnOps{
		&structs.TxnOp{
			Service: &structs.TxnServiceOp{
				Verb: api.ServiceSet,
				Node: "node1",
				Service: structs.NodeService{
					ID:      "api1",
					Service: "api",
				},
			},
		},
		&structs.TxnOp{
			Service: &structs.TxnServiceOp{
				Verb: api.ServiceSet,
				Node: "node1",
				Service: structs.NodeService{
					ID:      "redis1",
					Service: "redis",
				},
			},
		},
		&structs.TxnOp{
			Service: &structs.TxnServiceOp{
				Verb: api.ServiceSet,
				Node: "node1",
				Service: structs.NodeService{
					Kind:    structs.ServiceKindConnectProxy,
					ID:      "redis1-proxy",
					Service: "redis-proxy",
				},
			},
		},
	}

	expected := []stream.Event{
		stream.Event{
			Topic: stream.Topic_ServiceHealthConnect,
			Index: 3,
			Key:   "api",
			Payload: &stream.Event_ServiceHealth{
				ServiceHealth: &stream.ServiceHealthUpdate{
					Op: stream.CatalogOp_Register,
					CheckServiceNode: &stream.CheckServiceNode{
						Node: &stream.Node{
							Node:      "node1",
							ID:        nodeID,
							Address:   "1.2.3.4",
							RaftIndex: stream.RaftIndex{CreateIndex: 1, ModifyIndex: 1},
						},
						Service: &stream.NodeService{
							ID:      "api1",
							Service: "api",
							Address: "1.1.1.1",
							Port:    8080,
							Connect: stream.ServiceConnect{
								Native: true,
							},
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
						},
					},
				},
			},
		},
		stream.Event{
			Topic: stream.Topic_ServiceHealthConnect,
			Index: 3,
			Key:   "redis",
			Payload: &stream.Event_ServiceHealth{
				ServiceHealth: &stream.ServiceHealthUpdate{
					Op: stream.CatalogOp_Register,
					CheckServiceNode: &stream.CheckServiceNode{
						Node: &stream.Node{
							Node:      "node1",
							ID:        nodeID,
							Address:   "1.2.3.4",
							RaftIndex: stream.RaftIndex{CreateIndex: 1, ModifyIndex: 1},
						},
						Service: &stream.NodeService{
							Kind:    structs.ServiceKindConnectProxy,
							ID:      "redis1-proxy",
							Service: "redis-proxy",
							Address: "2.2.2.2",
							Port:    9999,
							Proxy: stream.ConnectProxyConfig{
								DestinationServiceName: "redis",
							},
							RaftIndex: stream.RaftIndex{CreateIndex: 3, ModifyIndex: 3},
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
						},
					},
				},
			},
		},
	}

	// Check the output and make sure we only get events for the native service
	// and connect.
	{
		tx := s.db.Txn(false)
		events, err := s.TxnEvents(tx, 3, ops)
		require.NoError(err)

		// Filter out only the events with the connect topic.
		var connectEvents []stream.Event
		for _, event := range events {
			if event.Topic == stream.Topic_ServiceHealthConnect {
				connectEvents = append(connectEvents, event)
			}
		}
		require.Equal(expected, connectEvents)
		tx.Abort()
	}
}
