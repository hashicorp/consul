// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package state

import (
	"context"
	crand "crypto/rand"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/acl"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib/stringslice"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/types"
)

func makeRandomNodeID(t *testing.T) types.NodeID {
	id, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	return types.NodeID(id)
}

func TestStateStore_GetNodeID(t *testing.T) {
	s := testStateStore(t)

	_, out, err := s.GetNodeID(types.NodeID("wrongId"), nil, "")
	if err == nil || out != nil || !strings.Contains(err.Error(), "node lookup by ID failed: index error: UUID (without hyphens) must be") {
		t.Errorf("want an error, nil value, err:=%q ; out:=%+v", err.Error(), out)
	}
	_, out, err = s.GetNodeID(types.NodeID("0123456789abcdefghijklmnopqrstuvwxyz"), nil, "")
	if err == nil || out != nil || !strings.Contains(err.Error(), "node lookup by ID failed: index error: invalid UUID") {
		t.Errorf("want an error, nil value, err:=%q ; out:=%+v", err, out)
	}

	_, out, err = s.GetNodeID(types.NodeID("00a916bc-a357-4a19-b886-59419fcee50Z"), nil, "")
	if err == nil || out != nil || !strings.Contains(err.Error(), "node lookup by ID failed: index error: invalid UUID") {
		t.Errorf("want an error, nil value, err:=%q ; out:=%+v", err, out)
	}

	_, out, err = s.GetNodeID(types.NodeID("00a916bc-a357-4a19-b886-59419fcee506"), nil, "")
	if err != nil || out != nil {
		t.Errorf("do not want any error nor returned value, err:=%q ; out:=%+v", err, out)
	}

	nodeID := types.NodeID("00a916bc-a357-4a19-b886-59419fceeaaa")
	req := &structs.RegisterRequest{
		ID:      nodeID,
		Node:    "node1",
		Address: "1.2.3.4",
	}
	require.NoError(t, s.EnsureRegistration(1, req))

	_, out, err = s.GetNodeID(nodeID, nil, "")
	require.NoError(t, err)
	if out == nil || out.ID != nodeID {
		t.Fatalf("out should not be nil and contain nodeId, but was:=%#v", out)
	}

	// Case insensitive lookup should work as well
	_, out, err = s.GetNodeID(types.NodeID("00a916bC-a357-4a19-b886-59419fceeAAA"), nil, "")
	require.NoError(t, err)
	if out == nil || out.ID != nodeID {
		t.Fatalf("out should not be nil and contain nodeId, but was:=%#v", out)
	}
}

func TestStateStore_GetNode(t *testing.T) {
	assertExists := func(t *testing.T, s *Store, node, peerName string, expectIndex uint64) {
		idx, out, err := s.GetNode(node, nil, peerName)
		require.NoError(t, err)
		require.NotNil(t, out)
		require.Equal(t, expectIndex, idx)
		require.Equal(t, strings.ToLower(node), out.Node)
		require.Equal(t, strings.ToLower(peerName), out.PeerName)
	}
	assertNotExist := func(t *testing.T, s *Store, node, peerName string) {
		idx, out, err := s.GetNode(node, nil, peerName)
		require.NoError(t, err)
		require.Nil(t, out)
		require.Equal(t, uint64(0), idx)
	}

	t.Run("default peer", func(t *testing.T) {
		s := testStateStore(t)

		// initially does not exist
		assertNotExist(t, s, "node1", "")

		// Create it
		testRegisterNode(t, s, 1, "node1")

		// now exists
		assertExists(t, s, "node1", "", 1)

		// Case insensitive lookup should work as well
		assertExists(t, s, "NoDe1", "", 1)
	})

	t.Run("random peer", func(t *testing.T) {
		s := testStateStore(t)

		// initially do not exist
		assertNotExist(t, s, "node1", "")
		assertNotExist(t, s, "node1", "my-peer")

		// Create one with no peer, and one with a peer to test a peer-name crossing issue.
		testRegisterNode(t, s, 1, "node1")
		testRegisterNodeOpts(t, s, 2, "node1", func(n *structs.Node) error {
			n.PeerName = "my-peer"
			return nil
		})

		// now exist
		assertExists(t, s, "node1", "", 1)
		assertExists(t, s, "node1", "my-peer", 2)

		// Case insensitive lookup should work as well
		assertExists(t, s, "NoDe1", "", 1)
		assertExists(t, s, "NoDe1", "my-peer", 2)
	})
}

func TestStateStore_ensureNoNodeWithSimilarNameTxn(t *testing.T) {
	t.Parallel()
	s := testStateStore(t)

	nodeID := makeRandomNodeID(t)
	req := &structs.RegisterRequest{
		ID:              nodeID,
		Node:            "node1",
		Address:         "1.2.3.4",
		TaggedAddresses: map[string]string{"hello": "world"},
		NodeMeta:        map[string]string{"somekey": "somevalue"},
		Check: &structs.HealthCheck{
			Node:    "node1",
			CheckID: structs.SerfCheckID,
			Status:  api.HealthPassing,
		},
	}
	require.NoError(t, s.EnsureRegistration(1, req))
	req = &structs.RegisterRequest{
		ID:      types.NodeID(""),
		Node:    "node2",
		Address: "10.0.0.1",
		Check: &structs.HealthCheck{
			Node:    "node2",
			CheckID: structs.SerfCheckID,
			Status:  api.HealthPassing,
		},
	}
	require.NoError(t, s.EnsureRegistration(2, req))

	tx := s.db.WriteTxnRestore()
	defer tx.Abort()

	node := &structs.Node{
		ID:      makeRandomNodeID(t),
		Node:    "NOdE1", // Name is similar but case is different
		Address: "2.3.4.5",
	}

	// Lets conflict with node1 (has an ID)
	require.Error(t, ensureNoNodeWithSimilarNameTxn(tx, node, false),
		"Should return an error since another name with similar name exists")
	require.Error(t, ensureNoNodeWithSimilarNameTxn(tx, node, true),
		"Should return an error since another name with similar name exists")

	// Lets conflict with node without ID
	node.Node = "NoDe2"
	require.Error(t, ensureNoNodeWithSimilarNameTxn(tx, node, false),
		"Should return an error since another name with similar name exists")
	require.NoError(t, ensureNoNodeWithSimilarNameTxn(tx, node, true),
		"Should not clash with another similar node name without ID")

	// Set node1's Serf health to failing and replace it.
	newNode := &structs.Node{
		ID:      makeRandomNodeID(t),
		Node:    "node1",
		Address: "2.3.4.5",
	}
	require.Error(t, ensureNoNodeWithSimilarNameTxn(tx, newNode, false),
		"Should return an error since the previous node is still healthy")

	require.NoError(t, s.ensureCheckTxn(tx, 5, false, &structs.HealthCheck{
		Node:    "node1",
		CheckID: structs.SerfCheckID,
		Status:  api.HealthCritical,
	}))
	require.NoError(t, ensureNoNodeWithSimilarNameTxn(tx, newNode, false))
}

func TestStateStore_EnsureRegistration(t *testing.T) {
	t.Parallel()

	run := func(t *testing.T, peerName string) {
		s := testStateStore(t)
		// Start with just a node.
		nodeID := makeRandomNodeID(t)

		makeReq := func(f func(*structs.RegisterRequest)) *structs.RegisterRequest {
			req := &structs.RegisterRequest{
				ID:              nodeID,
				Node:            "node1",
				Address:         "1.2.3.4",
				TaggedAddresses: map[string]string{"hello": "world"},
				NodeMeta:        map[string]string{"somekey": "somevalue"},
				PeerName:        peerName,
				Locality:        &structs.Locality{Region: "us-west-1", Zone: "us-west-1a"},
			}
			if f != nil {
				f(req)
			}
			return req
		}

		verifyNode := func(t *testing.T) {
			node := &structs.Node{
				ID:              nodeID,
				Node:            "node1",
				Address:         "1.2.3.4",
				Partition:       structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
				TaggedAddresses: map[string]string{"hello": "world"},
				Meta:            map[string]string{"somekey": "somevalue"},
				RaftIndex:       structs.RaftIndex{CreateIndex: 1, ModifyIndex: 1},
				PeerName:        peerName,
				Locality:        &structs.Locality{Region: "us-west-1", Zone: "us-west-1a"},
			}

			_, out, err := s.GetNode("node1", nil, peerName)
			require.NoError(t, err)
			require.Equal(t, node, out)

			_, out2, err := s.GetNodeID(nodeID, nil, peerName)
			require.NoError(t, err)
			require.NotNil(t, out2)
			require.Equal(t, out, out2)
		}
		verifyService := func(t *testing.T) {
			svcmap := map[string]*structs.NodeService{
				"redis1": {
					ID:             "redis1",
					Service:        "redis",
					Address:        "1.1.1.1",
					Port:           8080,
					Tags:           []string{"primary"},
					Weights:        &structs.Weights{Passing: 1, Warning: 1},
					RaftIndex:      structs.RaftIndex{CreateIndex: 2, ModifyIndex: 2},
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					PeerName:       peerName,
					Locality:       &structs.Locality{Region: "us-west-1", Zone: "us-west-1a"},
				},
			}

			idx, out, err := s.NodeServices(nil, "node1", nil, peerName)
			require.NoError(t, err)
			require.Equal(t, uint64(2), idx)
			require.Equal(t, svcmap, out.Services)

			idx, r, err := s.NodeService(nil, "node1", "redis1", nil, peerName)
			require.NoError(t, err)
			require.Equal(t, uint64(2), idx)
			require.Equal(t, svcmap["redis1"], r)

			exp := svcmap["redis1"].ToServiceNode("node1")
			exp.ID = nodeID

			// lookup service by node name
			idx, sn, err := s.ServiceNode("", "node1", "redis1", nil, peerName)
			require.NoError(t, err)
			require.Equal(t, uint64(2), idx)
			require.Equal(t, exp, sn)

			// lookup service by node ID
			idx, sn, err = s.ServiceNode(string(nodeID), "", "redis1", nil, peerName)
			require.NoError(t, err)
			require.Equal(t, uint64(2), idx)
			require.Equal(t, exp, sn)

			// lookup service by invalid node
			_, _, err = s.ServiceNode("", "invalid-node", "redis1", nil, peerName)
			testutil.RequireErrorContains(t, err, "node not found")

			// lookup service without node name or ID
			_, _, err = s.ServiceNode("", "", "redis1", nil, peerName)
			testutil.RequireErrorContains(t, err, "Node ID or name required to lookup the service")
		}
		verifyCheck := func(t *testing.T) {
			checks := structs.HealthChecks{
				&structs.HealthCheck{
					Node:           "node1",
					CheckID:        "check1",
					Name:           "check",
					Status:         "critical",
					RaftIndex:      structs.RaftIndex{CreateIndex: 3, ModifyIndex: 3},
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					PeerName:       peerName,
				},
			}

			idx, out, err := s.NodeChecks(nil, "node1", nil, peerName)
			require.NoError(t, err)
			require.Equal(t, uint64(3), idx)
			require.Equal(t, checks, out)

			idx, c, err := s.NodeCheck("node1", "check1", nil, peerName)
			require.NoError(t, err)
			require.Equal(t, uint64(3), idx)
			require.Equal(t, checks[0], c)
		}
		verifyChecks := func(t *testing.T) {
			checks := structs.HealthChecks{
				&structs.HealthCheck{
					Node:           "node1",
					CheckID:        "check1",
					Name:           "check",
					Status:         "critical",
					RaftIndex:      structs.RaftIndex{CreateIndex: 3, ModifyIndex: 3},
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					PeerName:       peerName,
				},
				&structs.HealthCheck{
					Node:           "node1",
					CheckID:        "check2",
					Name:           "check",
					Status:         "critical",
					ServiceID:      "redis1",
					ServiceName:    "redis",
					ServiceTags:    []string{"primary"},
					RaftIndex:      structs.RaftIndex{CreateIndex: 4, ModifyIndex: 4},
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					PeerName:       peerName,
				},
			}

			idx, out, err := s.NodeChecks(nil, "node1", nil, peerName)
			require.NoError(t, err)
			require.Equal(t, uint64(4), idx)
			require.Equal(t, checks, out)
		}

		testutil.RunStep(t, "add a node", func(t *testing.T) {
			req := makeReq(nil)
			require.NoError(t, s.EnsureRegistration(1, req))

			// Retrieve the node and verify its contents.
			verifyNode(t)
		})

		testutil.RunStep(t, "add a node with invalid meta", func(t *testing.T) {
			// Add in a invalid service definition with too long Key value for Meta
			req := makeReq(func(req *structs.RegisterRequest) {
				req.Service = &structs.NodeService{
					ID:       "redis1",
					Service:  "redis",
					Address:  "1.1.1.1",
					Port:     8080,
					Meta:     map[string]string{strings.Repeat("a", 129): "somevalue"},
					Tags:     []string{"primary"},
					PeerName: peerName,
					Locality: &structs.Locality{Region: "us-west-1", Zone: "us-west-1a"},
				}
			})
			testutil.RequireErrorContains(t, s.EnsureRegistration(9, req), `Key is too long (limit: 128 characters)`)
		})

		// Add in a service definition.
		testutil.RunStep(t, "add a service definition", func(t *testing.T) {
			req := makeReq(func(req *structs.RegisterRequest) {
				req.Service = &structs.NodeService{
					ID:       "redis1",
					Service:  "redis",
					Address:  "1.1.1.1",
					Port:     8080,
					Tags:     []string{"primary"},
					Weights:  &structs.Weights{Passing: 1, Warning: 1},
					PeerName: peerName,
					Locality: &structs.Locality{Region: "us-west-1", Zone: "us-west-1a"},
				}
			})
			require.NoError(t, s.EnsureRegistration(2, req))

			// Verify that the service got registered.
			verifyNode(t)
			verifyService(t)
		})

		// Add in a top-level check.
		testutil.RunStep(t, "add a top level check", func(t *testing.T) {
			req := makeReq(func(req *structs.RegisterRequest) {
				req.Service = &structs.NodeService{
					ID:       "redis1",
					Service:  "redis",
					Address:  "1.1.1.1",
					Port:     8080,
					Tags:     []string{"primary"},
					Weights:  &structs.Weights{Passing: 1, Warning: 1},
					PeerName: peerName,
					Locality: &structs.Locality{Region: "us-west-1", Zone: "us-west-1a"},
				}
				req.Check = &structs.HealthCheck{
					Node:     "node1",
					CheckID:  "check1",
					Name:     "check",
					PeerName: peerName,
				}
			})
			require.NoError(t, s.EnsureRegistration(3, req))

			// Verify that the check got registered.
			verifyNode(t)
			verifyService(t)
			verifyCheck(t)
		})

		// Add a service check which should populate the ServiceName
		// and ServiceTags fields in the response.
		testutil.RunStep(t, "add a service check", func(t *testing.T) {
			req := makeReq(func(req *structs.RegisterRequest) {
				req.Service = &structs.NodeService{
					ID:       "redis1",
					Service:  "redis",
					Address:  "1.1.1.1",
					Port:     8080,
					Tags:     []string{"primary"},
					Weights:  &structs.Weights{Passing: 1, Warning: 1},
					PeerName: peerName,
					Locality: &structs.Locality{Region: "us-west-1", Zone: "us-west-1a"},
				}
				req.Check = &structs.HealthCheck{
					Node:     "node1",
					CheckID:  "check1",
					Name:     "check",
					PeerName: peerName,
				}
				req.Checks = structs.HealthChecks{
					&structs.HealthCheck{
						Node:      "node1",
						CheckID:   "check2",
						Name:      "check",
						ServiceID: "redis1",
						PeerName:  peerName,
					},
				}
			})
			require.NoError(t, s.EnsureRegistration(4, req))

			// Verify that the additional check got registered.
			verifyNode(t)
			verifyService(t)
			verifyChecks(t)
		})

		// Try to register a check for some other node (top-level check).
		testutil.RunStep(t, "try to register a check for some other node via the top level check", func(t *testing.T) {
			req := makeReq(func(req *structs.RegisterRequest) {
				req.Service = &structs.NodeService{
					ID:       "redis1",
					Service:  "redis",
					Address:  "1.1.1.1",
					Port:     8080,
					Tags:     []string{"primary"},
					Weights:  &structs.Weights{Passing: 1, Warning: 1},
					PeerName: peerName,
				}
				req.Check = &structs.HealthCheck{
					Node:     "nope",
					CheckID:  "check1",
					Name:     "check",
					PeerName: peerName,
				}
				req.Checks = structs.HealthChecks{
					&structs.HealthCheck{
						Node:      "node1",
						CheckID:   "check2",
						Name:      "check",
						ServiceID: "redis1",
						PeerName:  peerName,
					},
				}
			})
			testutil.RequireErrorContains(t, s.EnsureRegistration(5, req), `does not match node`)
			verifyNode(t)
			verifyService(t)
			verifyChecks(t)
		})

		testutil.RunStep(t, "try to register a check for some other node via the checks array", func(t *testing.T) {
			// Try to register a check for some other node (checks array).
			req := makeReq(func(req *structs.RegisterRequest) {
				req.Service = &structs.NodeService{
					ID:       "redis1",
					Service:  "redis",
					Address:  "1.1.1.1",
					Port:     8080,
					Tags:     []string{"primary"},
					Weights:  &structs.Weights{Passing: 1, Warning: 1},
					PeerName: peerName,
				}
				req.Checks = structs.HealthChecks{
					&structs.HealthCheck{
						Node:           "nope",
						CheckID:        "check2",
						Name:           "check",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
						PeerName:       peerName,
					},
				}
			})
			testutil.RequireErrorContains(t, s.EnsureRegistration(6, req), `does not match node`)
			verifyNode(t)
			verifyService(t)
			verifyChecks(t)
		})

		testutil.RunStep(t, "NodeService with WatchSet", func(t *testing.T) {
			ws := memdb.NewWatchSet()

			_, _, err := s.NodeService(ws, "node1", "watch1", nil, peerName)
			require.NoError(t, err)

			req := makeReq(func(req *structs.RegisterRequest) {
				req.Service = &structs.NodeService{
					ID:       "watch1",
					Service:  "redis",
					Address:  "1.1.1.1",
					Port:     8080,
					Tags:     []string{"primary"},
					Weights:  &structs.Weights{Passing: 1, Warning: 1},
					PeerName: peerName,
				}
			})
			require.NoError(t, s.EnsureRegistration(7, req))

			select {
			case <-ws.WatchCh(context.Background()):
			case <-time.After(100 * time.Millisecond):
				t.Fatal("WatchSet did not trigger after service registration")
			}
		})
	}

	t.Run("default peer", func(t *testing.T) {
		run(t, structs.DefaultPeerKeyword)
	})

	t.Run("random peer", func(t *testing.T) {
		run(t, "my-peer")
	})
}

func TestStateStore_EnsureRegistration_Restore(t *testing.T) {
	const (
		nodeID   = "099eac9d-8e3e-464b-b3f5-8d7dcfcf9f71"
		nodeName = "node1"
	)

	run := func(t *testing.T, peerName string) {
		verifyNode := func(t *testing.T, s *Store, nodeLookup string, expectIdx uint64) {
			idx, out, err := s.GetNode(nodeLookup, nil, peerName)
			require.NoError(t, err)
			byID := false
			if out == nil {
				_, out, err = s.GetNodeID(types.NodeID(nodeLookup), nil, peerName)
				require.NoError(t, err)
				byID = true
			}

			require.NotNil(t, out)
			require.Equal(t, expectIdx, idx)

			require.Equal(t, "1.2.3.4", out.Address)
			if byID {
				require.Equal(t, nodeLookup, string(out.ID))
			} else {
				require.Equal(t, nodeLookup, out.Node)
			}
			require.Equal(t, peerName, out.PeerName)
			require.Equal(t, uint64(1), out.CreateIndex)
			require.Equal(t, uint64(1), out.ModifyIndex)
		}
		verifyService := func(t *testing.T, s *Store, nodeLookup string) {
			idx, out, err := s.NodeServices(nil, nodeLookup, nil, peerName)
			require.NoError(t, err)

			require.Len(t, out.Services, 1)
			require.Equal(t, uint64(2), idx)
			svc := out.Services["redis1"]

			require.Equal(t, "redis1", svc.ID)
			require.Equal(t, "redis", svc.Service)
			require.Equal(t, peerName, svc.PeerName)
			require.Equal(t, "1.1.1.1", svc.Address)
			require.Equal(t, 8080, svc.Port)
			require.Equal(t, uint64(2), svc.CreateIndex)
			require.Equal(t, uint64(2), svc.ModifyIndex)
		}
		verifyCheck := func(t *testing.T, s *Store) {
			idx, out, err := s.NodeChecks(nil, nodeName, nil, peerName)
			require.NoError(t, err)

			require.Len(t, out, 1)
			require.Equal(t, uint64(3), idx)

			c := out[0]

			require.Equal(t, strings.ToUpper(nodeName), c.Node)
			require.Equal(t, "check1", string(c.CheckID))
			require.Equal(t, "check", c.Name)
			require.Equal(t, peerName, c.PeerName)
			require.Equal(t, uint64(3), c.CreateIndex)
			require.Equal(t, uint64(3), c.ModifyIndex)
		}
		verifyChecks := func(t *testing.T, s *Store) {
			idx, out, err := s.NodeChecks(nil, nodeName, nil, peerName)
			require.NoError(t, err)

			require.Len(t, out, 2)
			require.Equal(t, uint64(4), idx)

			c1 := out[0]
			require.Equal(t, strings.ToUpper(nodeName), c1.Node)
			require.Equal(t, "check1", string(c1.CheckID))
			require.Equal(t, "check", c1.Name)
			require.Equal(t, peerName, c1.PeerName)
			require.Equal(t, uint64(3), c1.CreateIndex)
			require.Equal(t, uint64(3), c1.ModifyIndex)

			c2 := out[1]
			require.Equal(t, nodeName, c2.Node)
			require.Equal(t, "check2", string(c2.CheckID))
			require.Equal(t, "check", c2.Name)
			require.Equal(t, peerName, c2.PeerName)
			require.Equal(t, uint64(4), c2.CreateIndex)
			require.Equal(t, uint64(4), c2.ModifyIndex)
		}

		makeReq := func(f func(*structs.RegisterRequest)) *structs.RegisterRequest {
			req := &structs.RegisterRequest{
				ID:      types.NodeID(nodeID),
				Node:    nodeName,
				Address: "1.2.3.4",
				RaftIndex: structs.RaftIndex{
					CreateIndex: 1,
					ModifyIndex: 1,
				},
				PeerName: peerName,
			}
			if f != nil {
				f(req)
			}
			return req
		}

		s := testStateStore(t)

		// Start with just a node.
		testutil.RunStep(t, "add a node", func(t *testing.T) {
			req := makeReq(nil)
			restore := s.Restore()
			require.NoError(t, restore.Registration(1, req))
			require.NoError(t, restore.Commit())

			// Retrieve the node and verify its contents.
			verifyNode(t, s, nodeID, 1)
			verifyNode(t, s, nodeName, 1)
		})

		// Add in a service definition.
		testutil.RunStep(t, "add a service definition", func(t *testing.T) {
			req := makeReq(func(req *structs.RegisterRequest) {
				req.Service = &structs.NodeService{
					ID:      "redis1",
					Service: "redis",
					Address: "1.1.1.1",
					Port:    8080,
					Weights: &structs.Weights{Passing: 1, Warning: 1},
					RaftIndex: structs.RaftIndex{
						CreateIndex: 2,
						ModifyIndex: 2,
					},
					PeerName: peerName,
				}
			})
			restore := s.Restore()
			require.NoError(t, restore.Registration(2, req))
			require.NoError(t, restore.Commit())

			// Verify that the service got registered.
			verifyNode(t, s, nodeID, 2)
			verifyNode(t, s, nodeName, 2)
			verifyService(t, s, nodeID)
			verifyService(t, s, nodeName)
		})

		testutil.RunStep(t, "add a top-level check", func(t *testing.T) {
			// Add in a top-level check.
			//
			// Verify that node name references in checks are case-insensitive during
			// restore.
			req := makeReq(func(req *structs.RegisterRequest) {
				req.Service = &structs.NodeService{
					ID:      "redis1",
					Service: "redis",
					Address: "1.1.1.1",
					Port:    8080,
					Weights: &structs.Weights{Passing: 1, Warning: 1},
					RaftIndex: structs.RaftIndex{
						CreateIndex: 2,
						ModifyIndex: 2,
					},
					PeerName: peerName,
				}
				req.Check = &structs.HealthCheck{
					Node:    strings.ToUpper(nodeName),
					CheckID: "check1",
					Name:    "check",
					RaftIndex: structs.RaftIndex{
						CreateIndex: 3,
						ModifyIndex: 3,
					},
					PeerName: peerName,
				}
			})
			restore := s.Restore()
			require.NoError(t, restore.Registration(3, req))
			require.NoError(t, restore.Commit())

			// Verify that the check got registered.
			verifyNode(t, s, nodeID, 2)
			verifyNode(t, s, nodeName, 2)
			verifyService(t, s, nodeID)
			verifyService(t, s, nodeName)
			verifyCheck(t, s)
		})

		testutil.RunStep(t, "add another check via the slice", func(t *testing.T) {
			// Add in another check via the slice.
			req := makeReq(func(req *structs.RegisterRequest) {
				req.Service = &structs.NodeService{
					ID:      "redis1",
					Service: "redis",
					Address: "1.1.1.1",
					Port:    8080,
					Weights: &structs.Weights{Passing: 1, Warning: 1},
					RaftIndex: structs.RaftIndex{
						CreateIndex: 2,
						ModifyIndex: 2,
					},
					PeerName: peerName,
				}
				req.Check = &structs.HealthCheck{
					Node:    strings.ToUpper(nodeName),
					CheckID: "check1",
					Name:    "check",
					RaftIndex: structs.RaftIndex{
						CreateIndex: 3,
						ModifyIndex: 3,
					},
					PeerName: peerName,
				}
				req.Checks = structs.HealthChecks{
					&structs.HealthCheck{
						Node:    nodeName,
						CheckID: "check2",
						Name:    "check",
						RaftIndex: structs.RaftIndex{
							CreateIndex: 4,
							ModifyIndex: 4,
						},
						PeerName: peerName,
					},
				}
			})
			restore := s.Restore()
			require.NoError(t, restore.Registration(4, req))
			require.NoError(t, restore.Commit())

			// Verify that the additional check got registered.
			verifyNode(t, s, nodeID, 2)
			verifyNode(t, s, nodeName, 2)
			verifyService(t, s, nodeID)
			verifyService(t, s, nodeName)
			verifyChecks(t, s)
		})
	}

	t.Run("default peer", func(t *testing.T) {
		run(t, structs.DefaultPeerKeyword)
	})

	t.Run("random peer", func(t *testing.T) {
		run(t, "my-peer")
	})
}

func deprecatedEnsureNodeWithoutIDCanRegister(t *testing.T, s *Store, nodeName string, txIdx uint64) {
	// All the following is deprecated, and should be removed in future Consul versions
	in := &structs.Node{
		Node:    nodeName,
		Address: "1.1.1.9",
		Meta: map[string]string{
			"version": fmt.Sprint(txIdx),
		},
	}
	if err := s.EnsureNode(txIdx, in); err != nil {
		t.Fatalf("err: %s", err)
	}
	idx, out, err := s.GetNode(nodeName, nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != txIdx {
		t.Fatalf("index should be %v, was: %v", txIdx, idx)
	}
	if out.Node != nodeName {
		t.Fatalf("unexpected result out = %v, nodeName supposed to be %s", out, nodeName)
	}
}

func TestStateStore_EnsureNodeDeprecated(t *testing.T) {
	s := testStateStore(t)

	firstNodeName := "node-without-id"
	deprecatedEnsureNodeWithoutIDCanRegister(t, s, firstNodeName, 1)

	newNodeID := types.NodeID("00a916bc-a357-4a19-b886-59419fcee50c")
	// With this request, we basically add a node ID to existing node
	// and change its address
	in := &structs.Node{
		ID:      newNodeID,
		Node:    firstNodeName,
		Address: "1.1.7.8",
	}
	if err := s.EnsureNode(4, in); err != nil {
		t.Fatalf("err: %v", err)
	}
	// Retrieve the node again
	idx, out, err := s.GetNode(firstNodeName, nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Node has updated information
	if idx != 4 || out.Node != firstNodeName || out.ID != newNodeID || out.Address != "1.1.7.8" {
		t.Fatalf("[DEPRECATED] bad node returned: %#v", out)
	}
	if out.CreateIndex != 1 || out.ModifyIndex != 4 {
		t.Fatalf("[DEPRECATED] bad CreateIndex/ModifyIndex returned: %#v", out)
	}

	// Now, lets update IP Address without providing any ID
	// Only name of node will be used to match
	in = &structs.Node{
		Node:    firstNodeName,
		Address: "1.1.7.10",
	}
	if err := s.EnsureNode(7, in); err != nil {
		t.Fatalf("err: %v", err)
	}
	// Retrieve the node again
	idx, out, err = s.GetNode(firstNodeName, nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Node has updated information, its ID has been removed (deprecated, but working)
	if idx != 7 || out.Node != firstNodeName || out.ID != "" || out.Address != "1.1.7.10" {
		t.Fatalf("[DEPRECATED] bad node returned: %#v", out)
	}
	if out.CreateIndex != 1 || out.ModifyIndex != 7 {
		t.Fatalf("[DEPRECATED] bad CreateIndex/ModifyIndex returned: %#v", out)
	}
}

func TestNodeRenamingNodes(t *testing.T) {
	s := testStateStore(t)

	nodeID1 := types.NodeID("b789bf0a-d96b-4f70-a4a6-ac5dfaece53d")
	nodeID2 := types.NodeID("27bee224-a4d7-45d0-9b8e-65b3c94a61ba")

	// Node1 with ID
	in1 := &structs.Node{
		ID:      nodeID1,
		Node:    "node1",
		Address: "1.1.1.1",
	}

	if err := s.EnsureNode(1, in1); err != nil {
		t.Fatalf("err: %s", err)
	}
	if err := s.EnsureCheck(2, &structs.HealthCheck{
		Node:    "node1",
		CheckID: structs.SerfCheckID,
		Status:  api.HealthPassing,
	}); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Node2 with ID
	in2 := &structs.Node{
		ID:      nodeID2,
		Node:    "node2",
		Address: "1.1.1.2",
	}

	if err := s.EnsureNode(3, in2); err != nil {
		t.Fatalf("err: %s", err)
	}
	if err := s.EnsureCheck(4, &structs.HealthCheck{
		Node:    "node2",
		CheckID: structs.SerfCheckID,
		Status:  api.HealthPassing,
	}); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Node3 without ID
	in3 := &structs.Node{
		Node:    "node3",
		Address: "1.1.1.3",
	}

	if err := s.EnsureNode(5, in3); err != nil {
		t.Fatalf("err: %s", err)
	}
	if err := s.EnsureCheck(6, &structs.HealthCheck{
		Node:    "node3",
		CheckID: structs.SerfCheckID,
		Status:  api.HealthPassing,
	}); err != nil {
		t.Fatalf("err: %s", err)
	}

	if _, node, err := s.GetNodeID(nodeID1, nil, ""); err != nil || node == nil || node.ID != nodeID1 {
		t.Fatalf("err: %s, node:= %+v", err, node)
	}

	if _, node, err := s.GetNodeID(nodeID2, nil, ""); err != nil && node == nil || node.ID != nodeID2 {
		t.Fatalf("err: %s", err)
	}

	// Renaming node2 into node1 should fail
	in2Modify := &structs.Node{
		ID:      nodeID2,
		Node:    "node1",
		Address: "1.1.1.2",
	}
	if err := s.EnsureNode(7, in2Modify); err == nil {
		t.Fatalf("Renaming node2 into node1 should fail")
	}

	// Conflict with case insensitive matching as well
	in2Modify = &structs.Node{
		ID:      nodeID2,
		Node:    "NoDe1",
		Address: "1.1.1.2",
	}
	if err := s.EnsureNode(8, in2Modify); err == nil {
		t.Fatalf("Renaming node2 into node1 should fail")
	}

	// Conflict with case insensitive on node without ID
	in2Modify = &structs.Node{
		ID:      nodeID2,
		Node:    "NoDe3",
		Address: "1.1.1.2",
	}
	if err := s.EnsureNode(9, in2Modify); err == nil {
		t.Fatalf("Renaming node2 into node1 should fail")
	}

	// No conflict, should work
	in2Modify = &structs.Node{
		ID:      nodeID2,
		Node:    "node2bis",
		Address: "1.1.1.2",
	}
	if err := s.EnsureNode(10, in2Modify); err != nil {
		t.Fatalf("Renaming node2 into node1 should not fail: " + err.Error())
	}

	// Retrieve the node again
	idx, out, err := s.GetNode("node2bis", nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Retrieve the node again
	idx2, out2, err := s.GetNodeID(nodeID2, nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if idx != idx2 {
		t.Fatalf("node should be the same")
	}

	if out.ID != out2.ID || out.Node != out2.Node {
		t.Fatalf("all should match")
	}
}

func TestStateStore_EnsureNode(t *testing.T) {
	s := testStateStore(t)

	// Fetching a non-existent node returns nil
	if _, node, err := s.GetNode("node1", nil, ""); node != nil || err != nil {
		t.Fatalf("expected (nil, nil), got: (%#v, %#v)", node, err)
	}

	// Create a node registration request
	in := &structs.Node{
		ID:      types.NodeID("cda916bc-a357-4a19-b886-59419fcee50c"),
		Node:    "node1",
		Address: "1.1.1.1",
	}

	// Ensure the node is registered in the db
	if err := s.EnsureNode(1, in); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Retrieve the node again
	idx, out, err := s.GetNode("node1", nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Correct node was returned
	if out.Node != "node1" || out.Address != "1.1.1.1" {
		t.Fatalf("bad node returned: %#v", out)
	}

	// Indexes are set properly
	if out.CreateIndex != 1 || out.ModifyIndex != 1 {
		t.Fatalf("bad node index: %#v", out)
	}
	if idx != 1 {
		t.Fatalf("bad index: %d", idx)
	}

	// Update the node registration
	in2 := &structs.Node{
		ID:      in.ID,
		Node:    in.Node,
		Address: "1.1.1.2",
	}
	if err := s.EnsureNode(2, in2); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Retrieve the node
	idx, out, err = s.GetNode("node1", nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Node and indexes were updated
	if out.CreateIndex != 1 || out.ModifyIndex != 2 || out.Address != "1.1.1.2" {
		t.Fatalf("bad: %#v", out)
	}
	if idx != 2 {
		t.Fatalf("bad index: %d", idx)
	}

	// Re-inserting data should not modify ModifiedIndex
	if err := s.EnsureNode(3, in2); err != nil {
		t.Fatalf("err: %s", err)
	}
	_, out, err = s.GetNode("node1", nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if out.CreateIndex != 1 || out.ModifyIndex != 2 || out.Address != "1.1.1.2" {
		t.Fatalf("node was modified: %#v", out)
	}

	// Node upsert preserves the create index
	in3 := &structs.Node{
		ID:      in.ID,
		Node:    in.Node,
		Address: "1.1.1.3",
	}
	if err := s.EnsureNode(3, in3); err != nil {
		t.Fatalf("err: %s", err)
	}
	idx, out, err = s.GetNode("node1", nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if out.CreateIndex != 1 || out.ModifyIndex != 3 || out.Address != "1.1.1.3" {
		t.Fatalf("node was modified: %#v", out)
	}
	if idx != 3 {
		t.Fatalf("bad index: %d", idx)
	}

	// Update index to 4, no change
	if err := s.EnsureNode(4, in); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Now try to add another node with the same ID
	in = &structs.Node{
		Node:    "node1-renamed",
		ID:      types.NodeID("cda916bc-a357-4a19-b886-59419fcee50c"),
		Address: "1.1.1.2",
	}
	if err := s.EnsureNode(6, in); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Retrieve the node
	_, out, err = s.GetNode("node1", nil, "")
	require.NoError(t, err)
	if out != nil {
		t.Fatalf("Node should not exist anymore: %+v", out)
	}

	idx, out, err = s.GetNode("node1-renamed", nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if out == nil {
		t.Fatalf("err: %s", err)
	}

	// Node and indexes were updated
	if out.CreateIndex != 1 || out.ModifyIndex != 6 || out.Address != "1.1.1.2" || out.Node != "node1-renamed" {
		t.Fatalf("bad: %#v", out)
	}
	if idx != 6 {
		t.Fatalf("bad index: %d", idx)
	}

	newNodeID := types.NodeID("d0347693-65cc-4d9f-a6e0-5025b2e6513f")

	// Set a Serf check on the new node to inform whether to allow changing ID
	if err := s.EnsureCheck(8, &structs.HealthCheck{
		Node:    "node1-renamed",
		CheckID: structs.SerfCheckID,
		Status:  api.HealthPassing,
	}); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Adding another node with same name should fail
	in = &structs.Node{
		Node:    "node1-renamed",
		ID:      newNodeID,
		Address: "1.1.1.7",
	}
	if err := s.EnsureNode(9, in); err == nil {
		t.Fatalf("There should be an error since node1-renamed already exists")
	}

	// Adding another node with same name but different case should fail
	in = &structs.Node{
		Node:    "Node1-RENAMED",
		ID:      newNodeID,
		Address: "1.1.1.7",
	}
	if err := s.EnsureNode(9, in); err == nil {
		t.Fatalf("err: %s", err)
	}

	// Lets add another valid node now
	in = &structs.Node{
		Node:    "Node1bis",
		ID:      newNodeID,
		Address: "1.1.1.7",
	}
	if err := s.EnsureNode(10, in); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Retrieve the node
	_, out, err = s.GetNode("Node1bis", nil, "")
	require.NoError(t, err)
	if out == nil {
		t.Fatalf("Node should exist, but was null")
	}

	// Renaming should fail
	in = &structs.Node{
		Node:    "Node1bis",
		ID:      newNodeID,
		Address: "1.1.1.7",
	}
	if err := s.EnsureNode(10, in); err != nil {
		t.Fatalf("err: %s", err)
	}

	idx, out, err = s.GetNode("Node1bis", nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Node and indexes were updated
	if out.ID != newNodeID || out.CreateIndex != 10 || out.ModifyIndex != 10 || out.Address != "1.1.1.7" || out.Node != "Node1bis" {
		t.Fatalf("bad: %#v", out)
	}
	if idx != 10 {
		t.Fatalf("bad index: %d", idx)
	}

	// Renaming to same value as first node should fail as well
	// Adding another node with same name but different case should fail
	in = &structs.Node{
		Node:    "node1-renamed",
		ID:      newNodeID,
		Address: "1.1.1.7",
	}
	if err := s.EnsureNode(11, in); err == nil {
		t.Fatalf("err: %s", err)
	}

	// It should fail also with different case
	in = &structs.Node{
		Node:    "Node1-Renamed",
		ID:      newNodeID,
		Address: "1.1.1.7",
	}
	if err := s.EnsureNode(11, in); err == nil {
		t.Fatalf("err: %s", err)
	}

	// But should work if names are different
	in = &structs.Node{
		Node:    "Node1-Renamed2",
		ID:      newNodeID,
		Address: "1.1.1.7",
	}
	if err := s.EnsureNode(12, in); err != nil {
		t.Fatalf("err: %s", err)
	}
	idx, out, err = s.GetNode("Node1-Renamed2", nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Node and indexes were updated
	if out.ID != newNodeID || out.CreateIndex != 10 || out.ModifyIndex != 12 || out.Address != "1.1.1.7" || out.Node != "Node1-Renamed2" {
		t.Fatalf("bad: %#v", out)
	}
	if idx != 12 {
		t.Fatalf("bad index: %d", idx)
	}

	// All the remaining tests are deprecated, please remove them on next Consul major release
	// See https://github.com/hashicorp/consul/pull/3983 for context

	// Deprecated behavior is following
	deprecatedEnsureNodeWithoutIDCanRegister(t, s, "new-node-without-id", 13)

	// Deprecated, but should work as well
	deprecatedEnsureNodeWithoutIDCanRegister(t, s, "new-node-without-id", 14)

	// All of this is deprecated as well, should be removed
	in = &structs.Node{
		Node:    "Node1-Renamed2",
		Address: "1.1.1.66",
	}
	if err := s.EnsureNode(15, in); err != nil {
		t.Fatalf("[DEPRECATED] it should work, err:= %q", err)
	}
	_, out, err = s.GetNode("Node1-Renamed2", nil, "")
	if err != nil {
		t.Fatalf("[DEPRECATED] err: %s", err)
	}
	if out.CreateIndex != 10 {
		t.Fatalf("[DEPRECATED] We expected to modify node previously added, but add index = %d for node %+v", out.CreateIndex, out)
	}
	if out.Address != "1.1.1.66" || out.ModifyIndex != 15 {
		t.Fatalf("[DEPRECATED] Node with newNodeID should have been updated, but was: %d with content := %+v", out.CreateIndex, out)
	}
}

func TestStateStore_GetNodes(t *testing.T) {
	s := testStateStore(t)

	// Listing with no results returns nil.
	ws := memdb.NewWatchSet()
	idx, res, err := s.Nodes(ws, nil, "")
	if idx != 0 || res != nil || err != nil {
		t.Fatalf("expected (0, nil, nil), got: (%d, %#v, %#v)", idx, res, err)
	}

	// Create some nodes in the state store.
	testRegisterNode(t, s, 0, "node0")
	testRegisterNode(t, s, 1, "node1")
	testRegisterNode(t, s, 2, "node2")
	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	// Retrieve the nodes.
	ws = memdb.NewWatchSet()
	idx, nodes, err := s.Nodes(ws, nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Highest index was returned.
	if idx != 2 {
		t.Fatalf("bad index: %d", idx)
	}

	// All nodes were returned.
	if n := len(nodes); n != 3 {
		t.Fatalf("bad node count: %d", n)
	}

	// Make sure the nodes match.
	for i, node := range nodes {
		if node.CreateIndex != uint64(i) || node.ModifyIndex != uint64(i) {
			t.Fatalf("bad node index: %d, %d", node.CreateIndex, node.ModifyIndex)
		}
		name := fmt.Sprintf("node%d", i)
		if node.Node != name {
			t.Fatalf("bad: %#v", node)
		}
	}

	// Make sure a node delete fires the watch.
	if watchFired(ws) {
		t.Fatalf("bad")
	}
	if err := s.DeleteNode(3, "node1", nil, ""); err != nil {
		t.Fatalf("err: %s", err)
	}
	if !watchFired(ws) {
		t.Fatalf("bad")
	}
}

func BenchmarkGetNodes(b *testing.B) {
	s := NewStateStore(nil)

	if err := s.EnsureNode(100, &structs.Node{Node: "foo", Address: "127.0.0.1"}); err != nil {
		b.Fatalf("err: %v", err)
	}
	if err := s.EnsureNode(101, &structs.Node{Node: "bar", Address: "127.0.0.2"}); err != nil {
		b.Fatalf("err: %v", err)
	}

	ws := memdb.NewWatchSet()
	for i := 0; i < b.N; i++ {
		s.Nodes(ws, nil, "")
	}
}

func TestStateStore_GetNodesByMeta(t *testing.T) {
	s := testStateStore(t)

	// Listing with no results returns nil
	ws := memdb.NewWatchSet()
	idx, res, err := s.NodesByMeta(ws, map[string]string{"somekey": "somevalue"}, nil, "")
	if idx != 0 || res != nil || err != nil {
		t.Fatalf("expected (0, nil, nil), got: (%d, %#v, %#v)", idx, res, err)
	}

	// Create some nodes in the state store.
	testRegisterNodeWithMeta(t, s, 0, "node0", map[string]string{"role": "client"})
	testRegisterNodeWithMeta(t, s, 1, "node1", map[string]string{"role": "client", "common": "1"})
	testRegisterNodeWithMeta(t, s, 2, "node2", map[string]string{"role": "server", "common": "1"})
	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	cases := []struct {
		filters map[string]string
		nodes   []string
	}{
		// Empty meta filter
		{
			filters: map[string]string{},
			nodes:   []string{},
		},
		// Simple meta filter
		{
			filters: map[string]string{"role": "server"},
			nodes:   []string{"node2"},
		},
		// Common meta filter
		{
			filters: map[string]string{"common": "1"},
			nodes:   []string{"node1", "node2"},
		},
		// Invalid meta filter
		{
			filters: map[string]string{"invalid": "nope"},
			nodes:   []string{},
		},
		// Multiple meta filters
		{
			filters: map[string]string{"role": "client", "common": "1"},
			nodes:   []string{"node1"},
		},
	}

	for _, tc := range cases {
		_, result, err := s.NodesByMeta(nil, tc.filters, nil, "")
		if err != nil {
			t.Fatalf("bad: %v", err)
		}

		if len(result) != len(tc.nodes) {
			t.Fatalf("bad: %v %v", result, tc.nodes)
		}

		for i, node := range result {
			if node.Node != tc.nodes[i] {
				t.Fatalf("bad: %v %v", node.Node, tc.nodes[i])
			}
		}
	}

	// Set up a watch.
	ws = memdb.NewWatchSet()
	_, _, err = s.NodesByMeta(ws, map[string]string{"role": "client"}, nil, "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make an unrelated modification and make sure the watch doesn't fire.
	testRegisterNodeWithMeta(t, s, 3, "node3", map[string]string{"foo": "bar"})
	if watchFired(ws) {
		t.Fatalf("bad")
	}

	// Change a watched key and make sure it fires.
	testRegisterNodeWithMeta(t, s, 4, "node0", map[string]string{"role": "different"})
	if !watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_NodeServices(t *testing.T) {
	s := testStateStore(t)

	// Register some nodes with similar IDs.
	{
		req := &structs.RegisterRequest{
			ID:      types.NodeID("40e4a748-2192-161a-0510-aaaaaaaaaaaa"),
			Node:    "node1",
			Address: "1.2.3.4",
		}
		require.NoError(t, s.EnsureRegistration(1, req))
	}
	{
		req := &structs.RegisterRequest{
			ID:      types.NodeID("40e4a748-2192-161a-0510-bbbbbbbbbbbb"),
			Node:    "node2",
			Address: "5.6.7.8",
		}
		require.NoError(t, s.EnsureRegistration(2, req))
	}

	// Look up by name.
	t.Run("Look up by name", func(t *testing.T) {
		{
			_, ns, err := s.NodeServices(nil, "node1", nil, "")
			require.NoError(t, err)
			require.NotNil(t, ns)
			require.Equal(t, "node1", ns.Node.Node)
		}
		{
			_, ns, err := s.NodeServices(nil, "node2", nil, "")
			require.NoError(t, err)
			require.NotNil(t, ns)
			require.Equal(t, "node2", ns.Node.Node)
		}
	})

	t.Run("Look up by UUID", func(t *testing.T) {
		{
			_, ns, err := s.NodeServices(nil, "40e4a748-2192-161a-0510-aaaaaaaaaaaa", nil, "")
			require.NoError(t, err)
			require.NotNil(t, ns)
			require.Equal(t, "node1", ns.Node.Node)
		}
		{
			_, ns, err := s.NodeServices(nil, "40e4a748-2192-161a-0510-bbbbbbbbbbbb", nil, "")
			require.NoError(t, err)
			require.NotNil(t, ns)
			require.Equal(t, "node2", ns.Node.Node)
		}
	})

	t.Run("Ambiguous prefix", func(t *testing.T) {
		_, ns, err := s.NodeServices(nil, "40e4a748-2192-161a-0510", nil, "")
		require.NoError(t, err)
		require.Nil(t, ns)
	})

	t.Run("Bad node", func(t *testing.T) {
		// Bad node, and not a UUID (should not get a UUID error).
		_, ns, err := s.NodeServices(nil, "nope", nil, "")
		require.NoError(t, err)
		require.Nil(t, ns)
	})

	t.Run("Specific prefix", func(t *testing.T) {
		_, ns, err := s.NodeServices(nil, "40e4a748-2192-161a-0510-bb", nil, "")
		require.NoError(t, err)
		require.NotNil(t, ns)
		require.Equal(t, "node2", ns.Node.Node)
	})
}

func TestStateStore_DeleteNode(t *testing.T) {
	s := testStateStore(t)

	// Create a node and register a service and health check with it.
	testRegisterNode(t, s, 0, "node1")
	testRegisterService(t, s, 1, "node1", "service1")
	testRegisterCheck(t, s, 2, "node1", "", "check1", api.HealthPassing)

	// Delete the node
	if err := s.DeleteNode(3, "node1", nil, ""); err != nil {
		t.Fatalf("err: %s", err)
	}

	// The node was removed
	if idx, n, err := s.GetNode("node1", nil, ""); err != nil || n != nil || idx != 3 {
		t.Fatalf("bad: %#v %d (err: %#v)", n, idx, err)
	}

	// Associated service was removed. Need to query this directly out of
	// the DB to make sure it is actually gone.
	tx := s.db.Txn(false)
	defer tx.Abort()
	services, err := tx.Get(tableServices, indexID, NodeServiceQuery{Node: "node1", Service: "service1"})
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if service := services.Next(); service != nil {
		t.Fatalf("bad: %#v", service)
	}

	// Associated health check was removed.
	checks, err := tx.Get(tableChecks, indexID, NodeCheckQuery{Node: "node1", CheckID: "check1"})
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if check := checks.Next(); check != nil {
		t.Fatalf("bad: %#v", check)
	}

	// Indexes were updated.
	assert.Equal(t, uint64(3), catalogChecksMaxIndex(tx, nil, ""))
	assert.Equal(t, uint64(3), catalogServicesMaxIndex(tx, nil, ""))
	assert.Equal(t, uint64(3), catalogNodesMaxIndex(tx, nil, ""))

	// Deleting a nonexistent node should be idempotent and not return
	// an error
	if err := s.DeleteNode(4, "node1", nil, ""); err != nil {
		t.Fatalf("err: %s", err)
	}
	assert.Equal(t, uint64(3), catalogNodesMaxIndex(s.db.ReadTxn(), nil, ""))
}

func TestStateStore_Node_Snapshot(t *testing.T) {
	s := testStateStore(t)

	// Create some nodes in the state store.
	testRegisterNode(t, s, 0, "node0")
	testRegisterNode(t, s, 1, "node1")
	testRegisterNode(t, s, 2, "node2")

	// Snapshot the nodes.
	snap := s.Snapshot()
	defer snap.Close()

	// Alter the real state store.
	testRegisterNode(t, s, 3, "node3")

	// Verify the snapshot.
	if idx := snap.LastIndex(); idx != 2 {
		t.Fatalf("bad index: %d", idx)
	}
	nodes, err := snap.Nodes()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	for i := 0; i < 3; i++ {
		node := nodes.Next().(*structs.Node)
		if node == nil {
			t.Fatalf("unexpected end of nodes")
		}

		if node.CreateIndex != uint64(i) || node.ModifyIndex != uint64(i) {
			t.Fatalf("bad node index: %d, %d", node.CreateIndex, node.ModifyIndex)
		}
		if node.Node != fmt.Sprintf("node%d", i) {
			t.Fatalf("bad: %#v", node)
		}
	}
	if nodes.Next() != nil {
		t.Fatalf("unexpected extra nodes")
	}
}

func TestStateStore_EnsureService(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	s := testStateStore(t)

	// Fetching services for a node with none returns nil.
	ws := memdb.NewWatchSet()
	idx, res, err := s.NodeServices(ws, "node1", nil, "")
	if err != nil || res != nil || idx != 0 {
		t.Fatalf("expected (0, nil, nil), got: (%d, %#v, %#v)", idx, res, err)
	}

	// Create the service registration.
	ns1 := &structs.NodeService{
		ID:             "service1",
		Service:        "redis",
		Tags:           []string{"prod"},
		Address:        "1.1.1.1",
		Port:           1111,
		Weights:        &structs.Weights{Passing: 1, Warning: 0},
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}

	// Creating a service without a node returns an error.
	if err := s.EnsureService(1, "node1", ns1); err != ErrMissingNode {
		t.Fatalf("expected %#v, got: %#v", ErrMissingNode, err)
	}
	if watchFired(ws) {
		t.Fatalf("bad")
	}

	// Register the nodes.
	testRegisterNode(t, s, 0, "node1")
	testRegisterNode(t, s, 1, "node2")
	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	// Service successfully registers into the state store.
	ws = memdb.NewWatchSet()
	_, _, err = s.NodeServices(ws, "node1", nil, "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if err = s.EnsureService(10, "node1", ns1); err != nil {
		t.Fatalf("err: %s", err)
	}
	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	// Register a similar service against both nodes.
	ns2 := *ns1
	ns2.ID = "service2"
	for _, n := range []string{"node1", "node2"} {
		if err := s.EnsureService(20, n, &ns2); err != nil {
			t.Fatalf("err: %s", err)
		}
	}

	// Register a different service on the bad node.
	ws = memdb.NewWatchSet()
	_, _, err = s.NodeServices(ws, "node1", nil, "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	ns3 := *ns1
	ns3.ID = "service3"
	if err := s.EnsureService(30, "node2", &ns3); err != nil {
		t.Fatalf("err: %s", err)
	}
	if watchFired(ws) {
		t.Fatalf("bad")
	}

	// Retrieve the services.
	ws = memdb.NewWatchSet()
	idx, out, err := s.NodeServices(ws, "node1", nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	// expect node1's max idx
	if idx != 20 {
		t.Fatalf("bad index: %d", idx)
	}

	// Only the services for the requested node are returned.
	if out == nil || len(out.Services) != 2 {
		t.Fatalf("bad services: %#v", out)
	}

	// Results match the inserted services and have the proper indexes set.
	expect1 := *ns1
	expect1.CreateIndex, expect1.ModifyIndex = 10, 10
	if svc := out.Services["service1"]; !reflect.DeepEqual(&expect1, svc) {
		t.Fatalf("bad: %#v", svc)
	}

	expect2 := ns2
	expect2.CreateIndex, expect2.ModifyIndex = 20, 20
	if svc := out.Services["service2"]; !reflect.DeepEqual(&expect2, svc) {
		t.Fatalf("bad: %#v %#v", ns2, svc)
	}

	// Index tables were updated.
	assert.Equal(t, uint64(30), catalogServicesMaxIndex(s.db.ReadTxn(), nil, ""))

	// Update a service registration.
	ns1.Address = "1.1.1.2"
	if err := s.EnsureService(40, "node1", ns1); err != nil {
		t.Fatalf("err: %s", err)
	}
	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	// Retrieve the service again and ensure it matches..
	idx, out, err = s.NodeServices(nil, "node1", nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 40 {
		t.Fatalf("bad index: %d", idx)
	}
	if out == nil || len(out.Services) != 2 {
		t.Fatalf("bad: %#v", out)
	}
	expect1.Address = "1.1.1.2"
	expect1.ModifyIndex = 40
	if svc := out.Services["service1"]; !reflect.DeepEqual(&expect1, svc) {
		t.Fatalf("bad: %#v", svc)
	}

	// Index tables were updated.
	assert.Equal(t, uint64(40), catalogServicesMaxIndex(s.db.ReadTxn(), nil, ""))
}

func TestStateStore_EnsureService_connectProxy(t *testing.T) {
	s := testStateStore(t)

	// Create the service registration.
	ns1 := &structs.NodeService{
		Kind:    structs.ServiceKindConnectProxy,
		ID:      "connect-proxy",
		Service: "connect-proxy",
		Address: "1.1.1.1",
		Port:    1111,
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 1,
		},
		Proxy:          structs.ConnectProxyConfig{DestinationServiceName: "foo"},
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}

	// Service successfully registers into the state store.
	testRegisterNode(t, s, 0, "node1")
	assert.Nil(t, s.EnsureService(10, "node1", ns1))

	// Retrieve and verify
	_, out, err := s.NodeServices(nil, "node1", nil, "")
	assert.Nil(t, err)
	assert.NotNil(t, out)
	assert.Len(t, out.Services, 1)

	expect1 := *ns1
	expect1.CreateIndex, expect1.ModifyIndex = 10, 10
	assert.Equal(t, &expect1, out.Services["connect-proxy"])
}

func TestStateStore_EnsureService_VirtualIPAssign(t *testing.T) {
	s := testStateStore(t)
	setVirtualIPFlags(t, s)

	// Create the service registration.
	entMeta := structs.DefaultEnterpriseMetaInDefaultPartition()
	ns1 := &structs.NodeService{
		ID:      "foo",
		Service: "foo",
		Address: "1.1.1.1",
		Port:    1111,
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 1,
		},
		Connect:        structs.ServiceConnect{Native: true},
		EnterpriseMeta: *entMeta,
	}

	// Service successfully registers into the state store.
	testRegisterNode(t, s, 0, "node1")
	require.NoError(t, s.EnsureService(10, "node1", ns1))

	// Make sure there's a virtual IP for the foo service.
	vip, err := s.VirtualIPForService(structs.PeeredServiceName{ServiceName: structs.ServiceName{Name: "foo"}})
	require.NoError(t, err)
	assert.Equal(t, "240.0.0.1", vip)

	// Retrieve and verify
	_, out, err := s.NodeServices(nil, "node1", nil, "")
	require.NoError(t, err)
	assert.NotNil(t, out)
	assert.Len(t, out.Services, 1)

	taggedAddress := out.Services["foo"].TaggedAddresses[structs.TaggedAddressVirtualIP]
	assert.Equal(t, vip, taggedAddress.Address)
	assert.Equal(t, ns1.Port, taggedAddress.Port)

	// Create the service registration.
	ns2 := &structs.NodeService{
		Kind:    structs.ServiceKindConnectProxy,
		ID:      "redis-proxy",
		Service: "redis-proxy",
		Address: "2.2.2.2",
		Port:    2222,
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 1,
		},
		Proxy:          structs.ConnectProxyConfig{DestinationServiceName: "redis"},
		EnterpriseMeta: *entMeta,
	}
	require.NoError(t, s.EnsureService(11, "node1", ns2))

	// Make sure the virtual IP has been incremented for the redis service.
	vip, err = s.VirtualIPForService(structs.PeeredServiceName{ServiceName: structs.ServiceName{Name: "redis"}})
	require.NoError(t, err)
	assert.Equal(t, "240.0.0.2", vip)

	// Retrieve and verify
	_, out, err = s.NodeServices(nil, "node1", nil, "")
	assert.Nil(t, err)
	assert.NotNil(t, out)
	assert.Len(t, out.Services, 2)

	taggedAddress = out.Services["redis-proxy"].TaggedAddresses[structs.TaggedAddressVirtualIP]
	assert.Equal(t, vip, taggedAddress.Address)
	assert.Equal(t, ns2.Port, taggedAddress.Port)

	// Delete the first service and make sure it no longer has a virtual IP assigned.
	require.NoError(t, s.DeleteService(12, "node1", "foo", entMeta, ""))
	vip, err = s.VirtualIPForService(structs.PeeredServiceName{ServiceName: structs.ServiceName{Name: "connect-proxy"}})
	require.NoError(t, err)
	assert.Equal(t, "", vip)

	// Register another instance of redis-proxy and make sure the virtual IP is unchanged.
	ns3 := &structs.NodeService{
		Kind:    structs.ServiceKindConnectProxy,
		ID:      "redis-proxy2",
		Service: "redis-proxy",
		Address: "3.3.3.3",
		Port:    3333,
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 1,
		},
		Proxy:          structs.ConnectProxyConfig{DestinationServiceName: "redis"},
		EnterpriseMeta: *entMeta,
	}
	require.NoError(t, s.EnsureService(13, "node1", ns3))

	// Make sure the virtual IP is unchanged for the redis service.
	vip, err = s.VirtualIPForService(structs.PeeredServiceName{ServiceName: structs.ServiceName{Name: "redis"}})
	require.NoError(t, err)
	assert.Equal(t, "240.0.0.2", vip)

	// Make sure the new instance has the same virtual IP.
	_, out, err = s.NodeServices(nil, "node1", nil, "")
	require.NoError(t, err)
	taggedAddress = out.Services["redis-proxy2"].TaggedAddresses[structs.TaggedAddressVirtualIP]
	assert.Equal(t, vip, taggedAddress.Address)
	assert.Equal(t, ns3.Port, taggedAddress.Port)

	// Register another service to take its virtual IP.
	ns4 := &structs.NodeService{
		Kind:    structs.ServiceKindConnectProxy,
		ID:      "web-proxy",
		Service: "web-proxy",
		Address: "4.4.4.4",
		Port:    4444,
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 1,
		},
		Proxy:          structs.ConnectProxyConfig{DestinationServiceName: "web"},
		EnterpriseMeta: *entMeta,
	}
	require.NoError(t, s.EnsureService(14, "node1", ns4))

	// Make sure the virtual IP has allocated from the previously freed service.
	vip, err = s.VirtualIPForService(structs.PeeredServiceName{ServiceName: structs.ServiceName{Name: "web"}})
	require.NoError(t, err)
	assert.Equal(t, "240.0.0.1", vip)

	// Retrieve and verify
	_, out, err = s.NodeServices(nil, "node1", nil, "")
	require.NoError(t, err)
	taggedAddress = out.Services["web-proxy"].TaggedAddresses[structs.TaggedAddressVirtualIP]
	assert.Equal(t, vip, taggedAddress.Address)
	assert.Equal(t, ns4.Port, taggedAddress.Port)

	// Register a node1 in another peer (technically this node would be imported
	// and stored through the peering stream handlers).
	testRegisterNodeOpts(t, s, 15, "node1", func(node *structs.Node) error {
		node.PeerName = "billing"
		return nil
	})
	// Register an identical service but imported from a peer
	ns5 := &structs.NodeService{
		Kind:    structs.ServiceKindConnectProxy,
		ID:      "web-proxy",
		Service: "web-proxy",
		Address: "4.4.4.4",
		Port:    4444,
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 1,
		},
		Proxy:          structs.ConnectProxyConfig{DestinationServiceName: "web"},
		EnterpriseMeta: *entMeta,
		PeerName:       "billing",
	}
	require.NoError(t, s.EnsureService(15, "node1", ns5))

	// Make sure the virtual IP is different from the identically named local service.
	vip, err = s.VirtualIPForService(structs.PeeredServiceName{Peer: "billing", ServiceName: structs.ServiceName{Name: "web"}})
	require.NoError(t, err)
	assert.Equal(t, "240.0.0.3", vip)

	// Retrieve and verify
	_, out, err = s.NodeServices(nil, "node1", nil, "billing")
	require.NoError(t, err)
	taggedAddress = out.Services["web-proxy"].TaggedAddresses[structs.TaggedAddressVirtualIP]
	assert.Equal(t, vip, taggedAddress.Address)
	assert.Equal(t, ns5.Port, taggedAddress.Port)
}

func TestStateStore_AssignManualVirtualIPs(t *testing.T) {
	s := testStateStore(t)
	setVirtualIPFlags(t, s)

	// Attempt to assign manual virtual IPs to a service that doesn't exist - should be a no-op.
	psn := structs.PeeredServiceName{ServiceName: structs.ServiceName{Name: "foo", EnterpriseMeta: *acl.DefaultEnterpriseMeta()}}
	found, svcs, err := s.AssignManualServiceVIPs(0, psn, []string{"7.7.7.7", "8.8.8.8"})
	require.NoError(t, err)
	require.False(t, found)
	require.Empty(t, svcs)
	serviceVIP, err := s.ServiceManualVIPs(psn)
	require.NoError(t, err)
	require.Nil(t, serviceVIP)

	// Create the service registration.
	entMeta := structs.DefaultEnterpriseMetaInDefaultPartition()
	ns1 := &structs.NodeService{
		ID:             "foo",
		Service:        "foo",
		Address:        "1.1.1.1",
		Port:           1111,
		Connect:        structs.ServiceConnect{Native: true},
		EnterpriseMeta: *entMeta,
	}

	// Service successfully registers into the state store.
	testRegisterNode(t, s, 0, "node1")
	require.NoError(t, s.EnsureService(1, "node1", ns1))

	// Make sure there's a virtual IP for the foo service.
	vip, err := s.VirtualIPForService(psn)
	require.NoError(t, err)
	assert.Equal(t, "240.0.0.1", vip)

	// No manual IP should be set yet.
	serviceVIP, err = s.ServiceManualVIPs(psn)
	require.NoError(t, err)
	require.Equal(t, "0.0.0.1", serviceVIP.IP.String())
	require.Empty(t, serviceVIP.ManualIPs)

	// Attempt to assign manual virtual IPs again.
	found, svcs, err = s.AssignManualServiceVIPs(2, psn, []string{"7.7.7.7", "8.8.8.8"})
	require.NoError(t, err)
	require.True(t, found)
	require.Empty(t, svcs)
	serviceVIP, err = s.ServiceManualVIPs(psn)
	require.NoError(t, err)
	require.Equal(t, "0.0.0.1", serviceVIP.IP.String())
	require.Equal(t, serviceVIP.ManualIPs, []string{"7.7.7.7", "8.8.8.8"})

	// Register another service via config entry.
	s.EnsureConfigEntry(3, &structs.ServiceResolverConfigEntry{
		Kind: structs.ServiceResolver,
		Name: "bar",
	})

	psn2 := structs.PeeredServiceName{ServiceName: structs.ServiceName{Name: "bar"}}
	vip, err = s.VirtualIPForService(psn2)
	require.NoError(t, err)
	assert.Equal(t, "240.0.0.2", vip)

	// Attempt to assign manual virtual IPs for bar, with one IP overlapping with foo.
	// This should cause the ip to be removed from foo's list of manual IPs.
	found, svcs, err = s.AssignManualServiceVIPs(4, psn2, []string{"7.7.7.7", "9.9.9.9"})
	require.NoError(t, err)
	require.True(t, found)
	require.ElementsMatch(t, svcs, []structs.PeeredServiceName{psn})

	serviceVIP, err = s.ServiceManualVIPs(psn)
	require.NoError(t, err)
	require.Equal(t, "0.0.0.1", serviceVIP.IP.String())
	require.Equal(t, []string{"8.8.8.8"}, serviceVIP.ManualIPs)
	require.Equal(t, uint64(4), serviceVIP.ModifyIndex)

	serviceVIP, err = s.ServiceManualVIPs(psn2)
	require.NoError(t, err)
	require.Equal(t, "0.0.0.2", serviceVIP.IP.String())
	require.Equal(t, []string{"7.7.7.7", "9.9.9.9"}, serviceVIP.ManualIPs)
	require.Equal(t, uint64(4), serviceVIP.ModifyIndex)
}

func TestStateStore_EnsureService_ReassignFreedVIPs(t *testing.T) {
	s := testStateStore(t)
	setVirtualIPFlags(t, s)

	// Create the service registration.
	entMeta := structs.DefaultEnterpriseMetaInDefaultPartition()
	ns1 := &structs.NodeService{
		ID:      "foo",
		Service: "foo",
		Address: "1.1.1.1",
		Port:    1111,
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 1,
		},
		Connect:        structs.ServiceConnect{Native: true},
		EnterpriseMeta: *entMeta,
	}

	// Service successfully registers into the state store.
	testRegisterNode(t, s, 0, "node1")
	require.NoError(t, s.EnsureService(10, "node1", ns1))

	// Make sure there's a virtual IP for the foo service.
	vip, err := s.VirtualIPForService(structs.PeeredServiceName{ServiceName: structs.ServiceName{Name: "foo"}})
	require.NoError(t, err)
	assert.Equal(t, "240.0.0.1", vip)

	// Retrieve and verify
	_, out, err := s.NodeServices(nil, "node1", nil, "")
	require.NoError(t, err)
	assert.NotNil(t, out)

	taggedAddress := out.Services["foo"].TaggedAddresses[structs.TaggedAddressVirtualIP]
	assert.Equal(t, vip, taggedAddress.Address)
	assert.Equal(t, ns1.Port, taggedAddress.Port)

	// Create the service registration.
	ns2 := &structs.NodeService{
		Kind:    structs.ServiceKindTypical,
		ID:      "redis",
		Service: "redis",
		Address: "2.2.2.2",
		Port:    2222,
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 1,
		},
		Connect:        structs.ServiceConnect{Native: true},
		EnterpriseMeta: *entMeta,
	}
	require.NoError(t, s.EnsureService(11, "node1", ns2))

	// Make sure the virtual IP has been incremented for the redis service.
	vip, err = s.VirtualIPForService(structs.PeeredServiceName{ServiceName: structs.ServiceName{Name: "redis"}})
	require.NoError(t, err)
	assert.Equal(t, "240.0.0.2", vip)

	// Retrieve and verify
	_, out, err = s.NodeServices(nil, "node1", nil, "")
	assert.Nil(t, err)
	assert.NotNil(t, out)

	taggedAddress = out.Services["redis"].TaggedAddresses[structs.TaggedAddressVirtualIP]
	assert.Equal(t, vip, taggedAddress.Address)
	assert.Equal(t, ns2.Port, taggedAddress.Port)

	// Delete the last  service and make sure it no longer has a virtual IP assigned.
	require.NoError(t, s.DeleteService(12, "node1", "redis", entMeta, ""))
	vip, err = s.VirtualIPForService(structs.PeeredServiceName{ServiceName: structs.ServiceName{Name: "redis"}})
	require.NoError(t, err)
	assert.Equal(t, "", vip)

	// Register a new service, should end up with the freed 240.0.0.2 address.
	ns3 := &structs.NodeService{
		Kind:    structs.ServiceKindTypical,
		ID:      "backend",
		Service: "backend",
		Address: "2.2.2.2",
		Port:    2222,
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 1,
		},
		Connect:        structs.ServiceConnect{Native: true},
		EnterpriseMeta: *entMeta,
	}
	require.NoError(t, s.EnsureService(13, "node1", ns3))

	vip, err = s.VirtualIPForService(structs.PeeredServiceName{ServiceName: structs.ServiceName{Name: "backend"}})
	require.NoError(t, err)
	assert.Equal(t, "240.0.0.2", vip)

	// Retrieve and verify
	_, out, err = s.NodeServices(nil, "node1", nil, "")
	assert.Nil(t, err)
	assert.NotNil(t, out)

	taggedAddress = out.Services["backend"].TaggedAddresses[structs.TaggedAddressVirtualIP]
	assert.Equal(t, vip, taggedAddress.Address)
	assert.Equal(t, ns3.Port, taggedAddress.Port)

	// Create a new service, no more freed VIPs so it should go back to using the counter.
	ns4 := &structs.NodeService{
		Kind:    structs.ServiceKindTypical,
		ID:      "frontend",
		Service: "frontend",
		Address: "2.2.2.2",
		Port:    2222,
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 1,
		},
		Connect:        structs.ServiceConnect{Native: true},
		EnterpriseMeta: *entMeta,
	}
	require.NoError(t, s.EnsureService(14, "node1", ns4))

	// Make sure the virtual IP has been incremented for the frontend service.
	vip, err = s.VirtualIPForService(structs.PeeredServiceName{ServiceName: structs.ServiceName{Name: "frontend"}})
	require.NoError(t, err)
	assert.Equal(t, "240.0.0.3", vip)

	// Retrieve and verify
	_, out, err = s.NodeServices(nil, "node1", nil, "")
	assert.Nil(t, err)
	assert.NotNil(t, out)

	taggedAddress = out.Services["frontend"].TaggedAddresses[structs.TaggedAddressVirtualIP]
	assert.Equal(t, vip, taggedAddress.Address)
	assert.Equal(t, ns4.Port, taggedAddress.Port)
}

func TestStateStore_Services(t *testing.T) {
	s := testStateStore(t)

	// Listing with no results returns an empty list.
	ws := memdb.NewWatchSet()
	idx, services, err := s.Services(ws, &acl.EnterpriseMeta{}, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 0 {
		t.Fatalf("bad: %d", idx)
	}
	if len(services) != 0 {
		t.Fatalf("bad: %v", services)
	}

	// Register several nodes and services.
	testRegisterNode(t, s, 1, "node1")
	ns1 := &structs.NodeService{
		ID:      "service1",
		Service: "redis",
		Tags:    []string{"prod", "primary"},
		Address: "1.1.1.1",
		Port:    1111,
	}
	ns1.EnterpriseMeta.Normalize()
	ns1.Tags = []string{}
	ns1.Meta = map[string]string{}
	if err := s.EnsureService(2, "node1", ns1); err != nil {
		t.Fatalf("err: %s", err)
	}
	ns1Dogs := testRegisterService(t, s, 3, "node1", "dogs")
	ns1Dogs.Tags = []string{}
	ns1Dogs.Meta = map[string]string{}
	ns1Dogs.EnterpriseMeta.Normalize()

	testRegisterNode(t, s, 4, "node2")
	ns2 := &structs.NodeService{
		ID:      "service3",
		Service: "redis",
		Tags:    []string{"prod", "replica"},
		Address: "1.1.1.1",
		Port:    1111,
	}
	ns2.Tags = []string{}
	ns2.EnterpriseMeta.Normalize()
	ns2t.Meta = map[string]string{}
	if err := s.EnsureService(5, "node2", ns2); err != nil {
		t.Fatalf("err: %s", err)
	}
	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	// Pull all the services.
	ws = memdb.NewWatchSet()
	idx, services, err = s.Services(ws, &acl.EnterpriseMeta{}, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 5 {
		t.Fatalf("bad index: %d", idx)
	}

	// Verify the result.
	expected := structs.ServiceNodes{
		ns1Dogs.ToServiceNode("node1"),
		ns1.ToServiceNode("node1"),
		ns2.ToServiceNode("node2"),
	}
	assertDeepEqual(t, expected, services, cmpopts.IgnoreFields(structs.ServiceNode{}, "RaftIndex"))

	// Deleting a node with a service should fire the watch.
	if err := s.DeleteNode(6, "node1", nil, ""); err != nil {
		t.Fatalf("err: %s", err)
	}
	if !watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_ServicesByNodeMeta(t *testing.T) {
	s := testStateStore(t)

	ws := memdb.NewWatchSet()

	t.Run("Listing with no results returns nil", func(t *testing.T) {
		idx, res, err := s.ServicesByNodeMeta(ws, map[string]string{"somekey": "somevalue"}, nil, "")
		if idx != 0 || len(res) != 0 || err != nil {
			t.Fatalf("expected (0, nil, nil), got: (%d, %#v, %#v)", idx, res, err)
		}
	})

	// Create some nodes and services in the state store.
	node0 := &structs.Node{Node: "node0", Address: "127.0.0.1", Meta: map[string]string{"role": "client", "common": "1"}}
	if err := s.EnsureNode(0, node0); err != nil {
		t.Fatalf("err: %v", err)
	}
	node1 := &structs.Node{Node: "node1", Address: "127.0.0.1", Meta: map[string]string{"role": "server", "common": "1"}}
	if err := s.EnsureNode(1, node1); err != nil {
		t.Fatalf("err: %v", err)
	}
	ns1 := &structs.NodeService{
		ID:      "service1",
		Service: "redis",
		Tags:    []string{"prod", "primary"},
		Address: "1.1.1.1",
		Port:    1111,
	}
	ns1.EnterpriseMeta.Normalize()
	if err := s.EnsureService(2, "node0", ns1); err != nil {
		t.Fatalf("err: %s", err)
	}
	ns2 := &structs.NodeService{
		ID:      "service1",
		Service: "redis",
		Tags:    []string{"prod", "replica"},
		Address: "1.1.1.1",
		Port:    1111,
	}
	ns2.EnterpriseMeta.Normalize()
	if err := s.EnsureService(3, "node1", ns2); err != nil {
		t.Fatalf("err: %s", err)
	}
	if !watchFired(ws) {
		t.Fatalf("expected the watch to be triggered by the queries")
	}

	ws = memdb.NewWatchSet()

	t.Run("Filter the services by the first node's meta value", func(t *testing.T) {
		_, res, err := s.ServicesByNodeMeta(ws, map[string]string{"role": "client"}, nil, "")
		if err != nil {
			t.Fatalf("err: %s", err)
		}
		expected := []*structs.ServiceNode{
			ns1.ToServiceNode("node0"),
		}
		assertDeepEqual(t, res, expected, cmpopts.IgnoreFields(structs.ServiceNode{}, "RaftIndex"))
	})

	t.Run("Get all services using the common meta value", func(t *testing.T) {
		_, res, err := s.ServicesByNodeMeta(ws, map[string]string{"common": "1"}, nil, "")
		if err != nil {
			t.Fatalf("err: %s", err)
		}
		require.Len(t, res, 2)
		expected := []*structs.ServiceNode{
			ns1.ToServiceNode("node0"),
			ns2.ToServiceNode("node1"),
		}
		assertDeepEqual(t, res, expected, cmpopts.IgnoreFields(structs.ServiceNode{}, "RaftIndex"))
	})

	t.Run("Get an empty list for an invalid meta value", func(t *testing.T) {
		_, res, err := s.ServicesByNodeMeta(ws, map[string]string{"invalid": "nope"}, nil, "")
		if err != nil {
			t.Fatalf("err: %s", err)
		}
		var expected []*structs.ServiceNode
		assertDeepEqual(t, res, expected, cmpopts.IgnoreFields(structs.ServiceNode{}, "RaftIndex"))
	})

	t.Run("Get the first node's service instance using multiple meta filters", func(t *testing.T) {
		_, res, err := s.ServicesByNodeMeta(ws, map[string]string{"role": "client", "common": "1"}, nil, "")
		if err != nil {
			t.Fatalf("err: %s", err)
		}
		expected := []*structs.ServiceNode{
			ns1.ToServiceNode("node0"),
		}
		assertDeepEqual(t, res, expected, cmpopts.IgnoreFields(structs.ServiceNode{}, "RaftIndex"))
	})

	t.Run("Registering some unrelated node + service should not fire the watch.", func(t *testing.T) {
		testRegisterNode(t, s, 4, "nope")
		testRegisterService(t, s, 5, "nope", "nope")
		if watchFired(ws) {
			t.Fatalf("expected the watch to timeout and not be triggered")
		}
	})

	t.Run("Uses watchLimit to limit the number of watches", func(t *testing.T) {
		patchWatchLimit(t, 10)

		var idx uint64 = 6
		for i := 0; i < watchLimit+2; i++ {
			node := fmt.Sprintf("many%d", i)
			testRegisterNodeWithMeta(t, s, idx, node, map[string]string{"common": "1"})
			idx++
			testRegisterService(t, s, idx, node, "nope")
			idx++
		}

		// Now get a fresh watch, which will be forced to watch the whole
		// service table.
		ws := memdb.NewWatchSet()
		_, _, err := s.ServicesByNodeMeta(ws, map[string]string{"common": "1"}, nil, "")
		require.NoError(t, err)

		testRegisterService(t, s, idx, "nope", "more-nope")
		if !watchFired(ws) {
			t.Fatalf("expected the watch to timeout and not be triggered")
		}
	})
}

// patchWatchLimit package variable. Not safe for concurrent use. Do not use
// with t.Parallel.
func patchWatchLimit(t *testing.T, limit int) {
	oldLimit := watchLimit
	watchLimit = limit
	t.Cleanup(func() {
		watchLimit = oldLimit
	})
}

func TestStateStore_ServiceNodes(t *testing.T) {
	s := testStateStore(t)

	// Listing with no results returns an empty list.
	ws := memdb.NewWatchSet()
	idx, nodes, err := s.ServiceNodes(ws, "db", nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 0 {
		t.Fatalf("bad: %d", idx)
	}
	if len(nodes) != 0 {
		t.Fatalf("bad: %v", nodes)
	}

	// Create some nodes and services.
	if err := s.EnsureNode(10, &structs.Node{Node: "foo", Address: "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := s.EnsureNode(11, &structs.Node{Node: "bar", Address: "127.0.0.2"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := s.EnsureService(12, "foo", &structs.NodeService{ID: "api", Service: "api", Tags: nil, Address: "", Port: 5000}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := s.EnsureService(13, "bar", &structs.NodeService{ID: "api", Service: "api", Tags: nil, Address: "", Port: 5000}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := s.EnsureService(14, "foo", &structs.NodeService{ID: "db", Service: "db", Tags: []string{"primary"}, Address: "", Port: 8000}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := s.EnsureService(15, "bar", &structs.NodeService{ID: "db", Service: "db", Tags: []string{"replica"}, Address: "", Port: 8000}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := s.EnsureService(16, "bar", &structs.NodeService{ID: "db2", Service: "db", Tags: []string{"replica"}, Address: "", Port: 8001}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	// Read everything back.
	ws = memdb.NewWatchSet()
	idx, nodes, err = s.ServiceNodes(ws, "db", nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 16 {
		t.Fatalf("bad: %d", idx)
	}
	if len(nodes) != 3 {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[0].Node != "bar" {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[0].Address != "127.0.0.2" {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[0].ServiceID != "db" {
		t.Fatalf("bad: %v", nodes)
	}
	if !stringslice.Contains(nodes[0].ServiceTags, "replica") {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[0].ServicePort != 8000 {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[1].Node != "bar" {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[1].Address != "127.0.0.2" {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[1].ServiceID != "db2" {
		t.Fatalf("bad: %v", nodes)
	}
	if !stringslice.Contains(nodes[1].ServiceTags, "replica") {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[1].ServicePort != 8001 {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[2].Node != "foo" {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[2].Address != "127.0.0.1" {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[2].ServiceID != "db" {
		t.Fatalf("bad: %v", nodes)
	}
	if !stringslice.Contains(nodes[2].ServiceTags, "primary") {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[2].ServicePort != 8000 {
		t.Fatalf("bad: %v", nodes)
	}

	// Registering some unrelated node should not fire the watch.
	testRegisterNode(t, s, 17, "nope")
	if watchFired(ws) {
		t.Fatalf("bad")
	}

	// But removing a node with the "db" service should fire the watch.
	if err := s.DeleteNode(18, "bar", nil, ""); err != nil {
		t.Fatalf("err: %s", err)
	}
	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	// Overwhelm the node tracking.
	idx = 19
	patchWatchLimit(t, 10)
	for i := 0; i < watchLimit+2; i++ {
		node := fmt.Sprintf("many%d", i)
		if err := s.EnsureNode(idx, &structs.Node{Node: node, Address: "127.0.0.1"}); err != nil {
			t.Fatalf("err: %v", err)
		}
		if err := s.EnsureService(idx, node, &structs.NodeService{ID: "db", Service: "db", Port: 8000}); err != nil {
			t.Fatalf("err: %v", err)
		}
		idx++
	}

	// Now get a fresh watch, which will be forced to watch the whole nodes
	// table.
	ws = memdb.NewWatchSet()
	_, _, err = s.ServiceNodes(ws, "db", nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Registering some unrelated node should fire the watch now.
	testRegisterNode(t, s, idx, "more-nope")
	if !watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_ServiceTagNodes(t *testing.T) {
	s := testStateStore(t)

	// Listing with no results returns an empty list.
	ws := memdb.NewWatchSet()
	idx, nodes, err := s.ServiceTagNodes(ws, "db", []string{"primary"}, nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 0 {
		t.Fatalf("bad: %d", idx)
	}
	if len(nodes) != 0 {
		t.Fatalf("bad: %v", nodes)
	}

	// Create some nodes and services.
	if err := s.EnsureNode(15, &structs.Node{Node: "foo", Address: "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := s.EnsureNode(16, &structs.Node{Node: "bar", Address: "127.0.0.2"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := s.EnsureService(17, "foo", &structs.NodeService{ID: "db", Service: "db", Tags: []string{"primary"}, Address: "", Port: 8000}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := s.EnsureService(18, "foo", &structs.NodeService{ID: "db2", Service: "db", Tags: []string{"replica"}, Address: "", Port: 8001}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := s.EnsureService(19, "bar", &structs.NodeService{ID: "db", Service: "db", Tags: []string{"replica"}, Address: "", Port: 8000}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	// Read everything back.
	ws = memdb.NewWatchSet()
	idx, nodes, err = s.ServiceTagNodes(ws, "db", []string{"primary"}, nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 19 {
		t.Fatalf("bad: %v", idx)
	}
	if len(nodes) != 1 {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[0].Node != "foo" {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[0].Address != "127.0.0.1" {
		t.Fatalf("bad: %v", nodes)
	}
	if !stringslice.Contains(nodes[0].ServiceTags, "primary") {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[0].ServicePort != 8000 {
		t.Fatalf("bad: %v", nodes)
	}

	// Registering some unrelated node should not fire the watch.
	testRegisterNode(t, s, 20, "nope")
	if watchFired(ws) {
		t.Fatalf("bad")
	}

	// But removing a node with the "db:primary" service should fire the watch.
	if err := s.DeleteNode(21, "foo", nil, ""); err != nil {
		t.Fatalf("err: %s", err)
	}
	if !watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_ServiceTagNodes_MultipleTags(t *testing.T) {
	s := testStateStore(t)

	if err := s.EnsureNode(15, &structs.Node{Node: "foo", Address: "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := s.EnsureNode(16, &structs.Node{Node: "bar", Address: "127.0.0.2"}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := s.EnsureService(17, "foo", &structs.NodeService{ID: "db", Service: "db", Tags: []string{"primary", "v2"}, Address: "", Port: 8000}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := s.EnsureService(18, "foo", &structs.NodeService{ID: "db2", Service: "db", Tags: []string{"replica", "v2", "dev"}, Address: "", Port: 8001}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := s.EnsureService(19, "bar", &structs.NodeService{ID: "db", Service: "db", Tags: []string{"replica", "v2"}, Address: "", Port: 8000}); err != nil {
		t.Fatalf("err: %v", err)
	}

	idx, nodes, err := s.ServiceTagNodes(nil, "db", []string{"primary"}, nil, "")
	require.NoError(t, err)
	require.Equal(t, int(idx), 19)
	require.Len(t, nodes, 1)
	require.Equal(t, nodes[0].Node, "foo")
	require.Equal(t, nodes[0].Address, "127.0.0.1")
	require.Contains(t, nodes[0].ServiceTags, "primary")
	require.Equal(t, nodes[0].ServicePort, 8000)

	idx, nodes, err = s.ServiceTagNodes(nil, "db", []string{"v2"}, nil, "")
	require.NoError(t, err)
	require.Equal(t, int(idx), 19)
	require.Len(t, nodes, 3)

	// Test filtering on multiple tags
	idx, nodes, err = s.ServiceTagNodes(nil, "db", []string{"v2", "replica"}, nil, "")
	require.NoError(t, err)
	require.Equal(t, int(idx), 19)
	require.Len(t, nodes, 2)
	require.Contains(t, nodes[0].ServiceTags, "v2")
	require.Contains(t, nodes[0].ServiceTags, "replica")
	require.Contains(t, nodes[1].ServiceTags, "v2")
	require.Contains(t, nodes[1].ServiceTags, "replica")

	idx, nodes, err = s.ServiceTagNodes(nil, "db", []string{"dev"}, nil, "")
	require.NoError(t, err)
	require.Equal(t, int(idx), 19)
	require.Len(t, nodes, 1)
	require.Equal(t, nodes[0].Node, "foo")
	require.Equal(t, nodes[0].Address, "127.0.0.1")
	require.Contains(t, nodes[0].ServiceTags, "dev")
	require.Equal(t, nodes[0].ServicePort, 8001)
}

func TestStateStore_DeleteService(t *testing.T) {
	s := testStateStore(t)

	// Register a node with one service and a check.
	testRegisterNode(t, s, 1, "node1")
	testRegisterService(t, s, 2, "node1", "service1")
	testRegisterCheck(t, s, 3, "node1", "service1", "check1", api.HealthPassing)

	// register a node with a service on a cluster peer.
	testRegisterNodeOpts(t, s, 4, "node1", func(n *structs.Node) error {
		n.PeerName = "cluster-01"
		return nil
	})
	testRegisterServiceOpts(t, s, 5, "node1", "service1", func(service *structs.NodeService) {
		service.PeerName = "cluster-01"
	})

	wsPeer := memdb.NewWatchSet()
	_, ns, err := s.NodeServices(wsPeer, "node1", nil, "cluster-01")
	require.Len(t, ns.Services, 1)
	require.NoError(t, err)

	ws := memdb.NewWatchSet()
	_, ns, err = s.NodeServices(ws, "node1", nil, "")
	require.Len(t, ns.Services, 1)
	require.NoError(t, err)

	{
		// Delete the peered service.
		err = s.DeleteService(6, "node1", "service1", nil, "cluster-01")
		require.NoError(t, err)
		require.True(t, watchFired(wsPeer))
		_, kindServiceNames, err := s.ServiceNamesOfKind(nil, structs.ServiceKindTypical)
		require.NoError(t, err)
		require.Len(t, kindServiceNames, 1)
		require.Equal(t, "service1", kindServiceNames[0].Service.Name)
	}

	{
		// Delete the service.
		err = s.DeleteService(6, "node1", "service1", nil, "")
		require.NoError(t, err)
		require.True(t, watchFired(ws))
		_, kindServiceNames, err := s.ServiceNamesOfKind(nil, structs.ServiceKindTypical)
		require.NoError(t, err)
		require.Len(t, kindServiceNames, 0)
	}

	// Service doesn't exist.
	ws = memdb.NewWatchSet()
	_, ns, err = s.NodeServices(ws, "node1", nil, "")
	if err != nil || ns == nil || len(ns.Services) != 0 {
		t.Fatalf("bad: %#v (err: %#v)", ns, err)
	}

	// Check doesn't exist. Check using the raw DB so we can test
	// that it actually is removed in the state store.
	tx := s.db.Txn(false)
	defer tx.Abort()
	check, err := tx.First(tableChecks, indexID, NodeCheckQuery{Node: "node1", CheckID: "check1"})
	if err != nil || check != nil {
		t.Fatalf("bad: %#v (err: %s)", check, err)
	}

	// Index tables were updated.
	assert.Equal(t, uint64(6), catalogChecksMaxIndex(tx, nil, ""))
	assert.Equal(t, uint64(6), catalogServicesMaxIndex(tx, nil, ""))

	// Deleting a nonexistent service should be idempotent and not return an
	// error, nor fire a watch.
	if err := s.DeleteService(6, "node1", "service1", nil, ""); err != nil {
		t.Fatalf("err: %s", err)
	}
	assert.Equal(t, uint64(6), catalogServicesMaxIndex(tx, nil, ""))
	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_ConnectServiceNodes(t *testing.T) {
	s := testStateStore(t)

	// Listing with no results returns an empty list.
	ws := memdb.NewWatchSet()
	idx, nodes, err := s.ConnectServiceNodes(ws, "db", nil, "")
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(0))
	assert.Len(t, nodes, 0)

	// Create some nodes and services.
	assert.Nil(t, s.EnsureNode(10, &structs.Node{Node: "foo", Address: "127.0.0.1"}))
	assert.Nil(t, s.EnsureNode(11, &structs.Node{Node: "bar", Address: "127.0.0.2"}))
	assert.Nil(t, s.EnsureService(12, "foo", &structs.NodeService{ID: "db", Service: "db", Tags: nil, Address: "", Port: 5000}))
	assert.Nil(t, s.EnsureService(13, "bar", &structs.NodeService{ID: "api", Service: "api", Tags: nil, Address: "", Port: 5000}))
	assert.Nil(t, s.EnsureService(14, "foo", &structs.NodeService{Kind: structs.ServiceKindConnectProxy, ID: "proxy", Service: "proxy", Proxy: structs.ConnectProxyConfig{DestinationServiceName: "db"}, Port: 8000}))
	assert.Nil(t, s.EnsureService(15, "bar", &structs.NodeService{Kind: structs.ServiceKindConnectProxy, ID: "proxy", Service: "proxy", Proxy: structs.ConnectProxyConfig{DestinationServiceName: "db"}, Port: 8000}))
	assert.Nil(t, s.EnsureService(16, "bar", &structs.NodeService{ID: "native-db", Service: "db", Connect: structs.ServiceConnect{Native: true}}))
	assert.Nil(t, s.EnsureService(17, "bar", &structs.NodeService{ID: "db2", Service: "db", Tags: []string{"replica"}, Address: "", Port: 8001}))
	assert.True(t, watchFired(ws))

	// Read everything back.
	ws = memdb.NewWatchSet()
	idx, nodes, err = s.ConnectServiceNodes(ws, "db", nil, "")
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(17))
	assert.Len(t, nodes, 3)

	for _, n := range nodes {
		assert.True(t, n.ServiceKind == structs.ServiceKindConnectProxy ||
			n.ServiceConnect.Native,
			"either proxy or connect native")
	}

	// Registering some unrelated node should not fire the watch.
	testRegisterNode(t, s, 17, "nope")
	assert.False(t, watchFired(ws))

	// But removing a node with the "db" service should fire the watch.
	assert.Nil(t, s.DeleteNode(18, "bar", nil, ""))
	assert.True(t, watchFired(ws))
}

func TestStateStore_ConnectServiceNodes_Gateways(t *testing.T) {
	s := testStateStore(t)

	// Listing with no results returns an empty list.
	ws := memdb.NewWatchSet()
	idx, nodes, err := s.ConnectServiceNodes(ws, "db", nil, "")
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(0))
	assert.Len(t, nodes, 0)

	// Create some nodes and services.
	assert.Nil(t, s.EnsureNode(10, &structs.Node{Node: "foo", Address: "127.0.0.1"}))
	assert.Nil(t, s.EnsureNode(11, &structs.Node{Node: "bar", Address: "127.0.0.2"}))

	// Typical services
	assert.Nil(t, s.EnsureService(12, "foo", &structs.NodeService{ID: "db", Service: "db", Tags: nil, Address: "", Port: 5000}))
	assert.Nil(t, s.EnsureService(13, "bar", &structs.NodeService{ID: "api", Service: "api", Tags: nil, Address: "", Port: 5000}))
	assert.Nil(t, s.EnsureService(14, "bar", &structs.NodeService{ID: "db2", Service: "db", Tags: []string{"replica"}, Address: "", Port: 8001}))
	assert.False(t, watchFired(ws))

	// Register a sidecar for db
	assert.Nil(t, s.EnsureService(15, "foo", &structs.NodeService{Kind: structs.ServiceKindConnectProxy, ID: "proxy", Service: "proxy", Proxy: structs.ConnectProxyConfig{DestinationServiceName: "db"}, Port: 8000}))
	assert.True(t, watchFired(ws))

	// Reset WatchSet to ensure watch fires when associating db with gateway
	ws = memdb.NewWatchSet()
	_, _, err = s.ConnectServiceNodes(ws, "db", nil, "")
	assert.Nil(t, err)

	// Associate gateway with db
	assert.Nil(t, s.EnsureService(16, "bar", &structs.NodeService{Kind: structs.ServiceKindTerminatingGateway, ID: "gateway", Service: "gateway", Port: 443}))
	assert.Nil(t, s.EnsureConfigEntry(17, &structs.TerminatingGatewayConfigEntry{
		Kind: "terminating-gateway",
		Name: "gateway",
		Services: []structs.LinkedService{
			{
				Name: "db",
			},
		},
	}))
	assert.True(t, watchFired(ws))

	// Read everything back.
	ws = memdb.NewWatchSet()
	idx, nodes, err = s.ConnectServiceNodes(ws, "db", nil, "")
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(17))
	assert.Len(t, nodes, 2)

	// Check sidecar
	assert.Equal(t, structs.ServiceKindConnectProxy, nodes[0].ServiceKind)
	assert.Equal(t, "foo", nodes[0].Node)
	assert.Equal(t, "proxy", nodes[0].ServiceName)
	assert.Equal(t, "proxy", nodes[0].ServiceID)
	assert.Equal(t, "db", nodes[0].ServiceProxy.DestinationServiceName)
	assert.Equal(t, 8000, nodes[0].ServicePort)

	// Check gateway
	assert.Equal(t, structs.ServiceKindTerminatingGateway, nodes[1].ServiceKind)
	assert.Equal(t, "bar", nodes[1].Node)
	assert.Equal(t, "gateway", nodes[1].ServiceName)
	assert.Equal(t, "gateway", nodes[1].ServiceID)
	assert.Equal(t, 443, nodes[1].ServicePort)

	// Watch should fire when another gateway instance is registered
	assert.Nil(t, s.EnsureService(18, "foo", &structs.NodeService{Kind: structs.ServiceKindTerminatingGateway, ID: "gateway-2", Service: "gateway", Port: 443}))
	assert.True(t, watchFired(ws))

	// Reset WatchSet to ensure watch fires when deregistering gateway
	ws = memdb.NewWatchSet()
	_, _, err = s.ConnectServiceNodes(ws, "db", nil, "")
	assert.Nil(t, err)

	// Watch should fire when a gateway instance is deregistered
	assert.Nil(t, s.DeleteService(19, "bar", "gateway", nil, ""))
	assert.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	idx, nodes, err = s.ConnectServiceNodes(ws, "db", nil, "")
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(19))
	assert.Len(t, nodes, 2)

	// Check the new gateway
	assert.Equal(t, structs.ServiceKindTerminatingGateway, nodes[1].ServiceKind)
	assert.Equal(t, "foo", nodes[1].Node)
	assert.Equal(t, "gateway", nodes[1].ServiceName)
	assert.Equal(t, "gateway-2", nodes[1].ServiceID)
	assert.Equal(t, 443, nodes[1].ServicePort)

	// Index should not slide back after deleting all instances of the gateway
	assert.Nil(t, s.DeleteService(20, "foo", "gateway-2", nil, ""))
	assert.True(t, watchFired(ws))

	idx, nodes, err = s.ConnectServiceNodes(ws, "db", nil, "")
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(20))
	assert.Len(t, nodes, 1)

	// Ensure that remaining node is the proxy and not a gateway
	assert.Equal(t, structs.ServiceKindConnectProxy, nodes[0].ServiceKind)
	assert.Equal(t, "foo", nodes[0].Node)
	assert.Equal(t, "proxy", nodes[0].ServiceName)
	assert.Equal(t, "proxy", nodes[0].ServiceID)
	assert.Equal(t, 8000, nodes[0].ServicePort)
}

func TestStateStore_Service_Snapshot(t *testing.T) {
	s := testStateStore(t)

	// Register a node with two services.
	testRegisterNode(t, s, 0, "node1")
	ns := []*structs.NodeService{
		{
			ID:             "service1",
			Service:        "redis",
			Tags:           []string{"prod"},
			Address:        "1.1.1.1",
			Port:           1111,
			Weights:        &structs.Weights{Passing: 1, Warning: 0},
			EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
		},
		{
			ID:             "service2",
			Service:        "nomad",
			Tags:           []string{"dev"},
			Address:        "1.1.1.2",
			Port:           1112,
			Weights:        &structs.Weights{Passing: 1, Warning: 1},
			EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
		},
	}
	for i, svc := range ns {
		if err := s.EnsureService(uint64(i+1), "node1", svc); err != nil {
			t.Fatalf("err: %s", err)
		}
	}

	// Create a second node/service to make sure node filtering works. This
	// will affect the index but not the dump.
	testRegisterNode(t, s, 3, "node2")
	testRegisterService(t, s, 4, "node2", "service2")

	// Snapshot the service.
	snap := s.Snapshot()
	defer snap.Close()

	// Alter the real state store.
	testRegisterService(t, s, 5, "node2", "service3")

	// Verify the snapshot.
	if idx := snap.LastIndex(); idx != 4 {
		t.Fatalf("bad index: %d", idx)
	}
	services, err := snap.Services("node1", nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	for i := 0; i < len(ns); i++ {
		svc := services.Next().(*structs.ServiceNode)
		if svc == nil {
			t.Fatalf("unexpected end of services")
		}

		ns[i].CreateIndex, ns[i].ModifyIndex = uint64(i+1), uint64(i+1)
		if !reflect.DeepEqual(ns[i], svc.ToNodeService()) {
			t.Fatalf("bad: %#v != %#v", svc, ns[i])
		}
	}
	if services.Next() != nil {
		t.Fatalf("unexpected extra services")
	}
}

func TestStateStore_EnsureCheck(t *testing.T) {
	s := testStateStore(t)

	// Create a check associated with the node
	check := &structs.HealthCheck{
		Node:        "node1",
		CheckID:     "check1",
		Name:        "redis check",
		Status:      api.HealthPassing,
		Notes:       "test check",
		Output:      "aaa",
		ServiceID:   "service1",
		ServiceName: "redis",
	}

	// Creating a check without a node returns error
	if err := s.EnsureCheck(1, check); err != ErrMissingNode {
		t.Fatalf("expected %#v, got: %#v", ErrMissingNode, err)
	}

	// Register the node
	testRegisterNode(t, s, 1, "node1")

	// Creating a check with a bad services returns error
	if err := s.EnsureCheck(1, check); err != ErrMissingService {
		t.Fatalf("expected: %#v, got: %#v", ErrMissingService, err)
	}

	// Register the service
	testRegisterService(t, s, 2, "node1", "service1")

	// Inserting the check with the prerequisites succeeds
	if err := s.EnsureCheck(3, check); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Retrieve the check and make sure it matches
	idx, checks, err := s.NodeChecks(nil, "node1", nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 3 {
		t.Fatalf("bad index: %d", idx)
	}
	if len(checks) != 1 {
		t.Fatalf("wrong number of checks: %d", len(checks))
	}
	if !reflect.DeepEqual(checks[0], check) {
		t.Fatalf("bad: %#v", checks[0])
	}

	testCheckOutput := func(t *testing.T, expectedNodeIndex, expectedIndexForCheck uint64, outputTxt string) {
		t.Helper()
		// Check that we successfully updated
		idx, checks, err = s.NodeChecks(nil, "node1", nil, "")
		require.NoError(t, err)
		require.Equal(t, expectedNodeIndex, idx, "bad raft index")

		require.Len(t, checks, 1, "wrong number of checks")
		require.Equal(t, outputTxt, checks[0].Output, "wrong check output")
		require.Equal(t, uint64(3), checks[0].CreateIndex, "bad create index")
		require.Equal(t, expectedIndexForCheck, checks[0].ModifyIndex, "bad modify index")
	}
	// Do not really modify the health check content the health check
	check = &structs.HealthCheck{
		Node:        "node1",
		CheckID:     "check1",
		Name:        "redis check",
		Status:      api.HealthPassing,
		Notes:       "test check",
		Output:      "aaa",
		ServiceID:   "service1",
		ServiceName: "redis",
	}
	if err := s.EnsureCheck(4, check); err != nil {
		t.Fatalf("err: %s", err)
	}
	// Since there was no change to the check it won't actually have been updated
	// so the ModifyIndex index should still be 3
	testCheckOutput(t, 3, 3, check.Output)

	// Do modify the heathcheck
	check = &structs.HealthCheck{
		Node:        "node1",
		CheckID:     "check1",
		Name:        "redis check",
		Status:      api.HealthPassing,
		Notes:       "test check",
		Output:      "bbbmodified",
		ServiceID:   "service1",
		ServiceName: "redis",
	}
	if err := s.EnsureCheck(5, check); err != nil {
		t.Fatalf("err: %s", err)
	}
	testCheckOutput(t, 5, 5, "bbbmodified")

	// Index tables were updated
	assert.Equal(t, uint64(5), catalogChecksMaxIndex(s.db.ReadTxn(), nil, ""))
}

func TestStateStore_EnsureCheck_defaultStatus(t *testing.T) {
	s := testStateStore(t)

	// Register a node
	testRegisterNode(t, s, 1, "node1")

	// Create and register a check with no health status
	check := &structs.HealthCheck{
		Node:    "node1",
		CheckID: "check1",
		Status:  "",
	}
	if err := s.EnsureCheck(2, check); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Get the check again
	_, result, err := s.NodeChecks(nil, "node1", nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Check that the status was set to the proper default
	if len(result) != 1 || result[0].Status != api.HealthCritical {
		t.Fatalf("bad: %#v", result)
	}
}

func TestStateStore_NodeChecks(t *testing.T) {
	s := testStateStore(t)

	// Do an initial query for a node that doesn't exist.
	ws := memdb.NewWatchSet()
	idx, checks, err := s.NodeChecks(ws, "node1", nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 0 {
		t.Fatalf("bad: %d", idx)
	}
	if len(checks) != 0 {
		t.Fatalf("bad: %#v", checks)
	}

	// Create some nodes and checks.
	testRegisterNode(t, s, 0, "node1")
	testRegisterService(t, s, 1, "node1", "service1")
	testRegisterCheck(t, s, 2, "node1", "service1", "check1", api.HealthPassing)
	testRegisterCheck(t, s, 3, "node1", "service1", "check2", api.HealthPassing)
	testRegisterNode(t, s, 4, "node2")
	testRegisterService(t, s, 5, "node2", "service2")
	testRegisterCheck(t, s, 6, "node2", "service2", "check3", api.HealthPassing)
	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	// Try querying for all checks associated with node1
	ws = memdb.NewWatchSet()
	idx, checks, err = s.NodeChecks(ws, "node1", nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 6 {
		t.Fatalf("bad index: %d", idx)
	}
	if len(checks) != 2 || checks[0].CheckID != "check1" || checks[1].CheckID != "check2" {
		t.Fatalf("bad checks: %#v", checks)
	}

	// Creating some unrelated node should not fire the watch.
	testRegisterNode(t, s, 7, "node3")
	testRegisterCheck(t, s, 8, "node3", "", "check1", api.HealthPassing)
	if watchFired(ws) {
		t.Fatalf("bad")
	}

	// Try querying for all checks associated with node2
	ws = memdb.NewWatchSet()
	idx, checks, err = s.NodeChecks(ws, "node2", nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 8 {
		t.Fatalf("bad index: %d", idx)
	}
	if len(checks) != 1 || checks[0].CheckID != "check3" {
		t.Fatalf("bad checks: %#v", checks)
	}

	// Changing node2 should fire the watch.
	testRegisterCheck(t, s, 9, "node2", "service2", "check3", api.HealthCritical)
	if !watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_ServiceChecks(t *testing.T) {
	s := testStateStore(t)

	// Do an initial query for a service that doesn't exist.
	ws := memdb.NewWatchSet()
	idx, checks, err := s.ServiceChecks(ws, "service1", nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 0 {
		t.Fatalf("bad: %d", idx)
	}
	if len(checks) != 0 {
		t.Fatalf("bad: %#v", checks)
	}

	// Create some nodes and checks.
	testRegisterNode(t, s, 0, "node1")
	testRegisterService(t, s, 1, "node1", "service1")
	testRegisterCheck(t, s, 2, "node1", "service1", "check1", api.HealthPassing)
	testRegisterCheck(t, s, 3, "node1", "service1", "check2", api.HealthPassing)
	testRegisterNode(t, s, 4, "node2")
	testRegisterService(t, s, 5, "node2", "service2")
	testRegisterCheck(t, s, 6, "node2", "service2", "check3", api.HealthPassing)
	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	// Try querying for all checks associated with service1.
	ws = memdb.NewWatchSet()
	idx, checks, err = s.ServiceChecks(ws, "service1", nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 6 {
		t.Fatalf("bad index: %d", idx)
	}
	if len(checks) != 2 || checks[0].CheckID != "check1" || checks[1].CheckID != "check2" {
		t.Fatalf("bad checks: %#v", checks)
	}

	// Adding some unrelated service + check should not fire the watch.
	testRegisterService(t, s, 7, "node1", "service3")
	testRegisterCheck(t, s, 8, "node1", "service3", "check3", api.HealthPassing)
	if watchFired(ws) {
		t.Fatalf("bad")
	}

	// Updating a related check should fire the watch.
	testRegisterCheck(t, s, 9, "node1", "service1", "check2", api.HealthCritical)
	if !watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_ServiceChecksByNodeMeta(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	s := testStateStore(t)

	// Querying with no results returns nil.
	ws := memdb.NewWatchSet()
	idx, checks, err := s.ServiceChecksByNodeMeta(ws, "service1", nil, nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 0 {
		t.Fatalf("bad: %d", idx)
	}
	if len(checks) != 0 {
		t.Fatalf("bad: %#v", checks)
	}

	// Create some nodes and checks.
	testRegisterNodeWithMeta(t, s, 0, "node1", map[string]string{"somekey": "somevalue", "common": "1"})
	testRegisterService(t, s, 1, "node1", "service1")
	testRegisterCheck(t, s, 2, "node1", "service1", "check1", api.HealthPassing)
	testRegisterCheck(t, s, 3, "node1", "service1", "check2", api.HealthPassing)
	testRegisterNodeWithMeta(t, s, 4, "node2", map[string]string{"common": "1"})
	testRegisterService(t, s, 5, "node2", "service1")
	testRegisterCheck(t, s, 6, "node2", "service1", "check3", api.HealthPassing)
	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	cases := []struct {
		filters map[string]string
		checks  []string
	}{
		// Basic meta filter
		{
			filters: map[string]string{"somekey": "somevalue"},
			checks:  []string{"check1", "check2"},
		},
		// Common meta field
		{
			filters: map[string]string{"common": "1"},
			checks:  []string{"check1", "check2", "check3"},
		},
		// Invalid meta filter
		{
			filters: map[string]string{"invalid": "nope"},
			checks:  []string{},
		},
		// Multiple filters
		{
			filters: map[string]string{"somekey": "somevalue", "common": "1"},
			checks:  []string{"check1", "check2"},
		},
	}

	// Try querying for all checks associated with service1.
	idx = 7
	for _, tc := range cases {
		ws = memdb.NewWatchSet()
		_, checks, err := s.ServiceChecksByNodeMeta(ws, "service1", tc.filters, nil, "")
		if err != nil {
			t.Fatalf("err: %s", err)
		}
		if len(checks) != len(tc.checks) {
			t.Fatalf("bad checks: %#v", checks)
		}
		for i, check := range checks {
			if check.CheckID != types.CheckID(tc.checks[i]) {
				t.Fatalf("bad checks: %#v", checks)
			}
		}

		// Registering some unrelated node should not fire the watch.
		testRegisterNode(t, s, idx, fmt.Sprintf("nope%d", idx))
		idx++
		if watchFired(ws) {
			t.Fatalf("bad")
		}
	}

	// Overwhelm the node tracking.
	patchWatchLimit(t, 10)
	for i := 0; i < watchLimit+2; i++ {
		node := fmt.Sprintf("many%d", idx)
		testRegisterNodeWithMeta(t, s, idx, node, map[string]string{"common": "1"})
		idx++
		testRegisterService(t, s, idx, node, "service1")
		idx++
		testRegisterCheck(t, s, idx, node, "service1", "check1", api.HealthPassing)
		idx++
	}

	// Now get a fresh watch, which will be forced to watch the whole
	// node table.
	ws = memdb.NewWatchSet()
	_, _, err = s.ServiceChecksByNodeMeta(ws, "service1", map[string]string{"common": "1"}, nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Registering some unrelated node should now fire the watch.
	testRegisterNode(t, s, idx, "nope")
	if !watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_ChecksInState(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	s := testStateStore(t)

	// Querying with no results returns nil
	ws := memdb.NewWatchSet()
	idx, res, err := s.ChecksInState(ws, api.HealthPassing, nil, "")
	if idx != 0 || res != nil || err != nil {
		t.Fatalf("expected (0, nil, nil), got: (%d, %#v, %#v)", idx, res, err)
	}

	// Register a node with checks in varied states
	testRegisterNode(t, s, 0, "node1")
	testRegisterCheck(t, s, 1, "node1", "", "check1", api.HealthPassing)
	testRegisterCheck(t, s, 2, "node1", "", "check2", api.HealthCritical)
	testRegisterCheck(t, s, 3, "node1", "", "check3", api.HealthPassing)
	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	// Query the state store for passing checks.
	ws = memdb.NewWatchSet()
	_, checks, err := s.ChecksInState(ws, api.HealthPassing, nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Make sure we only get the checks which match the state
	if n := len(checks); n != 2 {
		t.Fatalf("expected 2 checks, got: %d", n)
	}
	if checks[0].CheckID != "check1" || checks[1].CheckID != "check3" {
		t.Fatalf("bad: %#v", checks)
	}
	if watchFired(ws) {
		t.Fatalf("bad")
	}

	// Changing the state of a check should fire the watch.
	testRegisterCheck(t, s, 4, "node1", "", "check1", api.HealthCritical)
	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	// HealthAny just returns everything.
	ws = memdb.NewWatchSet()
	_, checks, err = s.ChecksInState(ws, api.HealthAny, nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if n := len(checks); n != 3 {
		t.Fatalf("expected 3 checks, got: %d", n)
	}
	if watchFired(ws) {
		t.Fatalf("bad")
	}

	// Adding a new check should fire the watch.
	testRegisterCheck(t, s, 5, "node1", "", "check4", api.HealthCritical)
	if !watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_ChecksInStateByNodeMeta(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	s := testStateStore(t)

	// Querying with no results returns nil.
	ws := memdb.NewWatchSet()
	idx, res, err := s.ChecksInStateByNodeMeta(ws, api.HealthPassing, nil, nil, "")
	if idx != 0 || res != nil || err != nil {
		t.Fatalf("expected (0, nil, nil), got: (%d, %#v, %#v)", idx, res, err)
	}

	// Register a node with checks in varied states.
	testRegisterNodeWithMeta(t, s, 0, "node1", map[string]string{"somekey": "somevalue", "common": "1"})
	testRegisterCheck(t, s, 1, "node1", "", "check1", api.HealthPassing)
	testRegisterCheck(t, s, 2, "node1", "", "check2", api.HealthCritical)
	testRegisterNodeWithMeta(t, s, 3, "node2", map[string]string{"common": "1"})
	testRegisterCheck(t, s, 4, "node2", "", "check3", api.HealthPassing)
	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	cases := []struct {
		filters map[string]string
		state   string
		checks  []string
	}{
		// Basic meta filter, any status
		{
			filters: map[string]string{"somekey": "somevalue"},
			state:   api.HealthAny,
			checks:  []string{"check2", "check1"},
		},
		// Basic meta filter, only passing
		{
			filters: map[string]string{"somekey": "somevalue"},
			state:   api.HealthPassing,
			checks:  []string{"check1"},
		},
		// Common meta filter, any status
		{
			filters: map[string]string{"common": "1"},
			state:   api.HealthAny,
			checks:  []string{"check2", "check1", "check3"},
		},
		// Common meta filter, only passing
		{
			filters: map[string]string{"common": "1"},
			state:   api.HealthPassing,
			checks:  []string{"check1", "check3"},
		},
		// Invalid meta filter
		{
			filters: map[string]string{"invalid": "nope"},
			checks:  []string{},
		},
		// Multiple filters, any status
		{
			filters: map[string]string{"somekey": "somevalue", "common": "1"},
			state:   api.HealthAny,
			checks:  []string{"check2", "check1"},
		},
		// Multiple filters, only passing
		{
			filters: map[string]string{"somekey": "somevalue", "common": "1"},
			state:   api.HealthPassing,
			checks:  []string{"check1"},
		},
	}

	// Try querying for all checks associated with service1.
	idx = 5
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			ws = memdb.NewWatchSet()
			_, checks, err := s.ChecksInStateByNodeMeta(ws, tc.state, tc.filters, nil, "")
			require.NoError(t, err)

			var foundIDs []string
			for _, chk := range checks {
				foundIDs = append(foundIDs, string(chk.CheckID))
			}

			require.ElementsMatch(t, tc.checks, foundIDs)

			// Registering some unrelated node should not fire the watch.
			testRegisterNode(t, s, idx, fmt.Sprintf("nope%d", idx))
			idx++
			require.False(t, watchFired(ws))
		})
	}

	// Overwhelm the node tracking.
	patchWatchLimit(t, 10)
	for i := 0; i < watchLimit+2; i++ {
		node := fmt.Sprintf("many%d", idx)
		testRegisterNodeWithMeta(t, s, idx, node, map[string]string{"common": "1"})
		idx++
		testRegisterService(t, s, idx, node, "service1")
		idx++
		testRegisterCheck(t, s, idx, node, "service1", "check1", api.HealthPassing)
		idx++
	}

	// Now get a fresh watch, which will be forced to watch the whole
	// node table.
	ws = memdb.NewWatchSet()
	_, _, err = s.ChecksInStateByNodeMeta(ws, api.HealthPassing, map[string]string{"common": "1"}, nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Registering some unrelated node should now fire the watch.
	testRegisterNode(t, s, idx, "nope")
	if !watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_DeleteCheck(t *testing.T) {
	s := testStateStore(t)

	// Register a node and a node-level health check.
	testRegisterNode(t, s, 1, "node1")
	testRegisterCheck(t, s, 2, "node1", "", "check1", api.HealthPassing)
	testRegisterService(t, s, 2, "node1", "service1")

	// Make sure the check is there.
	ws := memdb.NewWatchSet()
	_, checks, err := s.NodeChecks(ws, "node1", nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if len(checks) != 1 {
		t.Fatalf("bad: %#v", checks)
	}

	ensureServiceVersion(t, s, ws, "service1", 2, 1)

	// Delete the check.
	if err := s.DeleteCheck(3, "node1", "check1", nil, ""); err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx, check, err := s.NodeCheck("node1", "check1", nil, ""); idx != 3 || err != nil || check != nil {
		t.Fatalf("Node check should have been deleted idx=%d, node=%v, err=%s", idx, check, err)
	}
	assert.Equal(t, uint64(3), catalogChecksMaxIndex(s.db.ReadTxn(), nil, ""))
	if !watchFired(ws) {
		t.Fatalf("bad")
	}
	// All services linked to this node should have their index updated
	ensureServiceVersion(t, s, ws, "service1", 3, 1)

	// Check is gone
	ws = memdb.NewWatchSet()
	_, checks, err = s.NodeChecks(ws, "node1", nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if len(checks) != 0 {
		t.Fatalf("bad: %#v", checks)
	}

	// Index tables were updated.
	assert.Equal(t, uint64(3), catalogChecksMaxIndex(s.db.ReadTxn(), nil, ""))

	// Deleting a nonexistent check should be idempotent and not return an
	// error.
	if err := s.DeleteCheck(4, "node1", "check1", nil, ""); err != nil {
		t.Fatalf("err: %s", err)
	}
	assert.Equal(t, uint64(3), catalogChecksMaxIndex(s.db.ReadTxn(), nil, ""))
	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func ensureServiceVersion(t *testing.T, s *Store, ws memdb.WatchSet, serviceID string, expectedIdx uint64, expectedSize int) {
	idx, services, err := s.ServiceNodes(ws, serviceID, nil, "")
	t.Helper()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != expectedIdx {
		t.Fatalf("bad: %d, expected %d", idx, expectedIdx)
	}
	if len(services) != expectedSize {
		t.Fatalf("expected size: %d, but was %d", expectedSize, len(services))
	}
}

// Ensure index exist, if expectedIndex = -1, ensure the index does not exists
func ensureIndexForService(t *testing.T, s *Store, serviceName string, expectedIndex uint64) {
	t.Helper()
	tx := s.db.Txn(false)
	defer tx.Abort()
	transaction, err := tx.First(tableIndex, "id", serviceIndexName(serviceName, nil, ""))
	if err == nil {
		if idx, ok := transaction.(*IndexEntry); ok {
			if expectedIndex != idx.Value {
				t.Fatalf("Expected index %d, but had %d for %s", expectedIndex, idx.Value, serviceName)
			}
			return
		}
	}
	if expectedIndex != 0 {
		t.Fatalf("Index for %s was expected but not found", serviceName)
	}
}

// TestStateStore_IndexIndependence test that changes on a given service does not impact the
// index of other services. It allows to have huge benefits for watches since
// watchers are notified ONLY when there are changes in the given service
func TestStateStore_IndexIndependence(t *testing.T) {
	s := testStateStore(t)

	// Querying with no matches gives an empty response
	ws := memdb.NewWatchSet()
	idx, res, err := s.CheckServiceNodes(ws, "service1", nil, "")
	if idx != 0 || res != nil || err != nil {
		t.Fatalf("expected (0, nil, nil), got: (%d, %#v, %#v)", idx, res, err)
	}

	// Register some nodes.
	testRegisterNode(t, s, 0, "node1")
	testRegisterNode(t, s, 1, "node2")

	// Register node-level checks. These should be the final result.
	testRegisterCheck(t, s, 2, "node1", "", "check1", api.HealthPassing)
	testRegisterCheck(t, s, 3, "node2", "", "check2", api.HealthPassing)

	// Register a service against the nodes.
	testRegisterService(t, s, 4, "node1", "service1")
	testRegisterService(t, s, 5, "node2", "service2")
	ensureServiceVersion(t, s, ws, "service2", 5, 1)

	// Register checks against the services.
	testRegisterCheck(t, s, 6, "node1", "service1", "check3", api.HealthPassing)
	testRegisterCheck(t, s, 7, "node2", "service2", "check4", api.HealthPassing)
	// Index must be updated when checks are updated
	ensureServiceVersion(t, s, ws, "service1", 6, 1)
	ensureServiceVersion(t, s, ws, "service2", 7, 1)

	if !watchFired(ws) {
		t.Fatalf("bad")
	}
	// We ensure the idx for service2 has not been changed
	testRegisterCheck(t, s, 8, "node2", "service2", "check4", api.HealthWarning)
	ensureServiceVersion(t, s, ws, "service2", 8, 1)
	testRegisterCheck(t, s, 9, "node2", "service2", "check4", api.HealthPassing)
	ensureServiceVersion(t, s, ws, "service2", 9, 1)

	// Add a new check on node1, while not on service, it should impact
	// indexes of all services running on node1, aka service1
	testRegisterCheck(t, s, 10, "node1", "", "check_node", api.HealthPassing)

	// Service2 should not be modified
	ensureServiceVersion(t, s, ws, "service2", 9, 1)
	// Service1 should be modified
	ensureServiceVersion(t, s, ws, "service1", 10, 1)

	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	testRegisterService(t, s, 11, "node1", "service_shared")
	ensureServiceVersion(t, s, ws, "service_shared", 11, 1)
	testRegisterService(t, s, 12, "node2", "service_shared")
	ensureServiceVersion(t, s, ws, "service_shared", 12, 2)

	testRegisterCheck(t, s, 13, "node2", "service_shared", "check_service_shared", api.HealthCritical)
	ensureServiceVersion(t, s, ws, "service_shared", 13, 2)
	testRegisterCheck(t, s, 14, "node2", "service_shared", "check_service_shared", api.HealthPassing)
	ensureServiceVersion(t, s, ws, "service_shared", 14, 2)

	s.DeleteCheck(15, "node2", types.CheckID("check_service_shared"), nil, "")
	ensureServiceVersion(t, s, ws, "service_shared", 15, 2)
	ensureIndexForService(t, s, "service_shared", 15)
	s.DeleteService(16, "node2", "service_shared", nil, "")
	ensureServiceVersion(t, s, ws, "service_shared", 16, 1)
	ensureIndexForService(t, s, "service_shared", 16)
	s.DeleteService(17, "node1", "service_shared", nil, "")
	ensureServiceVersion(t, s, ws, "service_shared", 17, 0)

	testRegisterService(t, s, 18, "node1", "service_new")

	// Since service does not exists anymore, its index should be that of
	// the last deleted service
	ensureServiceVersion(t, s, ws, "service_shared", 17, 0)

	// No index should exist anymore, it must have been garbage collected
	ensureIndexForService(t, s, "service_shared", 0)
	if !watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_ConnectQueryBlocking(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	tests := []struct {
		name                   string
		setupFn                func(s *Store)
		svc                    string
		wantBeforeResLen       int
		wantBeforeWatchSetSize int
		updateFn               func(s *Store)
		shouldFire             bool
		wantAfterIndex         uint64
		wantAfterResLen        int
		wantAfterWatchSetSize  int
	}{
		{
			name:             "not affected by non-connect-enabled target service registration",
			setupFn:          nil,
			svc:              "test",
			wantBeforeResLen: 0,
			// The connect index and gateway-services iterators are watched
			wantBeforeWatchSetSize: 2,
			updateFn: func(s *Store) {
				testRegisterService(t, s, 4, "node1", "test")
			},
			shouldFire:      false,
			wantAfterIndex:  4, // No results falls back to global service index
			wantAfterResLen: 0,
			// The connect index and gateway-services iterators are watched
			wantAfterWatchSetSize: 2,
		},
		{
			name: "not affected by non-connect-enabled target service de-registration",
			setupFn: func(s *Store) {
				testRegisterService(t, s, 4, "node1", "test")
			},
			svc:              "test",
			wantBeforeResLen: 0,
			// The connect index and gateway-services iterators are watched
			wantBeforeWatchSetSize: 2,
			updateFn: func(s *Store) {
				require.NoError(t, s.DeleteService(5, "node1", "test", nil, ""))
			},
			// Note that the old implementation would unblock in this case since it
			// always watched the target service's index even though some updates
			// there don't affect Connect result output. This doesn't matter much for
			// correctness but it causes pointless work.
			shouldFire:      false,
			wantAfterIndex:  5, // No results falls back to global service index
			wantAfterResLen: 0,
			// The connect index and gateway-services iterators are watched
			wantAfterWatchSetSize: 2,
		},
		{
			name:             "unblocks on first connect-native service registration",
			setupFn:          nil,
			svc:              "test",
			wantBeforeResLen: 0,
			// The connect index and gateway-services iterators are watched
			wantBeforeWatchSetSize: 2,
			updateFn: func(s *Store) {
				testRegisterConnectNativeService(t, s, 4, "node1", "test")
			},
			shouldFire:      true,
			wantAfterIndex:  4,
			wantAfterResLen: 1,
			// Should take the optimized path where we only watch the service index,
			// connect index iterator, and gateway-services iterator.
			wantAfterWatchSetSize: 3,
		},
		{
			name: "unblocks on subsequent connect-native service registration",
			setupFn: func(s *Store) {
				testRegisterConnectNativeService(t, s, 4, "node1", "test")
			},
			svc:              "test",
			wantBeforeResLen: 1,
			// Should take the optimized path where we only watch the service index,
			// connect index iterator, and gateway-services iterator.
			wantBeforeWatchSetSize: 3,
			updateFn: func(s *Store) {
				testRegisterConnectNativeService(t, s, 5, "node2", "test")
			},
			shouldFire:      true,
			wantAfterIndex:  5,
			wantAfterResLen: 2,
			// Should take the optimized path where we only watch the service index,
			// connect index iterator, and gateway-services iterator.
			wantAfterWatchSetSize: 3,
		},
		{
			name: "unblocks on connect-native service de-registration",
			setupFn: func(s *Store) {
				testRegisterConnectNativeService(t, s, 4, "node1", "test")
				testRegisterConnectNativeService(t, s, 5, "node2", "test")
			},
			svc:              "test",
			wantBeforeResLen: 2,
			// Should take the optimized path where we only watch the service index,
			// connect index iterator, and gateway-services iterator.
			wantBeforeWatchSetSize: 3,
			updateFn: func(s *Store) {
				require.NoError(t, s.DeleteService(6, "node2", "test", nil, ""))
			},
			shouldFire:      true,
			wantAfterIndex:  6,
			wantAfterResLen: 1,
			// Should take the optimized path where we only watch the service index,
			// connect index iterator, and gateway-services iterator.
			wantAfterWatchSetSize: 3,
		},
		{
			name: "unblocks on last connect-native service de-registration",
			setupFn: func(s *Store) {
				testRegisterConnectNativeService(t, s, 4, "node1", "test")
			},
			svc:              "test",
			wantBeforeResLen: 1,
			// Should take the optimized path where we only watch the service index,
			// connect index iterator, and gateway-services iterator.
			wantBeforeWatchSetSize: 3,
			updateFn: func(s *Store) {
				require.NoError(t, s.DeleteService(6, "node1", "test", nil, ""))
			},
			shouldFire:      true,
			wantAfterIndex:  6,
			wantAfterResLen: 0,
			// The connect index and gateway-services iterators are watched
			wantAfterWatchSetSize: 2,
		},
		{
			name:             "unblocks on first proxy service registration",
			setupFn:          nil,
			svc:              "test",
			wantBeforeResLen: 0,
			// The connect index and gateway-services iterators are watched
			wantBeforeWatchSetSize: 2,
			updateFn: func(s *Store) {
				testRegisterSidecarProxy(t, s, 4, "node1", "test")
			},
			shouldFire:      true,
			wantAfterIndex:  4,
			wantAfterResLen: 1,
			// Should take the optimized path where we only watch the service index,
			// connect index iterator, and gateway-services iterator.
			wantAfterWatchSetSize: 3,
		},
		{
			name: "unblocks on subsequent proxy service registration",
			setupFn: func(s *Store) {
				testRegisterSidecarProxy(t, s, 4, "node1", "test")
			},
			svc:              "test",
			wantBeforeResLen: 1,
			// Should take the optimized path where we only watch the service index,
			// connect index iterator, and gateway-services iterator.
			wantBeforeWatchSetSize: 3,
			updateFn: func(s *Store) {
				testRegisterSidecarProxy(t, s, 5, "node2", "test")
			},
			shouldFire:      true,
			wantAfterIndex:  5,
			wantAfterResLen: 2,
			// Should take the optimized path where we only watch the service index,
			// connect index iterator, and gateway-services iterator.
			wantAfterWatchSetSize: 3,
		},
		{
			name: "unblocks on proxy service de-registration",
			setupFn: func(s *Store) {
				testRegisterSidecarProxy(t, s, 4, "node1", "test")
				testRegisterSidecarProxy(t, s, 5, "node2", "test")
			},
			svc:              "test",
			wantBeforeResLen: 2,
			// Should take the optimized path where we only watch the service index,
			// connect index iterator, and gateway-services iterator.
			wantBeforeWatchSetSize: 3,
			updateFn: func(s *Store) {
				require.NoError(t, s.DeleteService(6, "node2", "test-sidecar-proxy", nil, ""))
			},
			shouldFire:      true,
			wantAfterIndex:  6,
			wantAfterResLen: 1,
			// Should take the optimized path where we only watch the service index,
			// connect index iterator, and gateway-services iterator.
			wantAfterWatchSetSize: 3,
		},
		{
			name: "unblocks on last proxy service de-registration",
			setupFn: func(s *Store) {
				testRegisterSidecarProxy(t, s, 4, "node1", "test")
			},
			svc:              "test",
			wantBeforeResLen: 1,
			// Should take the optimized path where we only watch the service index,
			// connect index iterator, and gateway-services iterator.
			wantBeforeWatchSetSize: 3,
			updateFn: func(s *Store) {
				require.NoError(t, s.DeleteService(6, "node1", "test-sidecar-proxy", nil, ""))
			},
			shouldFire:      true,
			wantAfterIndex:  6,
			wantAfterResLen: 0,
			// The connect index and gateway-services iterators are watched
			wantAfterWatchSetSize: 2,
		},
		{
			name: "unblocks on connect-native service health check change",
			setupFn: func(s *Store) {
				testRegisterConnectNativeService(t, s, 4, "node1", "test")
				testRegisterCheck(t, s, 6, "node1", "test", "check1", "passing")
			},
			svc:              "test",
			wantBeforeResLen: 1,
			// Should take the optimized path where we only watch the service index,
			// connect index iterator, and gateway-services iterator.
			wantBeforeWatchSetSize: 3,
			updateFn: func(s *Store) {
				testRegisterCheck(t, s, 7, "node1", "test", "check1", "critical")
			},
			shouldFire:      true,
			wantAfterIndex:  7,
			wantAfterResLen: 1, // critical filtering doesn't happen in the state store method.
			// Should take the optimized path where we only watch the service index,
			// connect index iterator, and gateway-services iterator.
			wantAfterWatchSetSize: 3,
		},
		{
			name: "unblocks on proxy service health check change",
			setupFn: func(s *Store) {
				testRegisterSidecarProxy(t, s, 4, "node1", "test")
				testRegisterCheck(t, s, 6, "node1", "test-sidecar-proxy", "check1", "passing")
			},
			svc:              "test",
			wantBeforeResLen: 1,
			// Should take the optimized path where we only watch the service index,
			// connect index iterator, and gateway-services iterator.
			wantBeforeWatchSetSize: 3,
			updateFn: func(s *Store) {
				testRegisterCheck(t, s, 7, "node1", "test-sidecar-proxy", "check1", "critical")
			},
			shouldFire:      true,
			wantAfterIndex:  7,
			wantAfterResLen: 1, // critical filtering doesn't happen in the state store method.
			// Should take the optimized path where we only watch the service index,
			// connect index iterator, and gateway-services iterator.
			wantAfterWatchSetSize: 3,
		},
		{
			name: "unblocks on connect-native node health check change",
			setupFn: func(s *Store) {
				testRegisterConnectNativeService(t, s, 4, "node1", "test")
				testRegisterCheck(t, s, 6, "node1", "", "check1", "passing")
			},
			svc:              "test",
			wantBeforeResLen: 1,
			// Should take the optimized path where we only watch the service index,
			// connect index iterator, and gateway-services iterator.
			wantBeforeWatchSetSize: 3,
			updateFn: func(s *Store) {
				testRegisterCheck(t, s, 7, "node1", "", "check1", "critical")
			},
			shouldFire:      true,
			wantAfterIndex:  7,
			wantAfterResLen: 1, // critical filtering doesn't happen in the state store method.
			// Should take the optimized path where we only watch the service index,
			// connect index iterator, and gateway-services iterator.
			wantAfterWatchSetSize: 3,
		},
		{
			name: "unblocks on proxy service health check change",
			setupFn: func(s *Store) {
				testRegisterSidecarProxy(t, s, 4, "node1", "test")
				testRegisterCheck(t, s, 6, "node1", "", "check1", "passing")
			},
			svc:              "test",
			wantBeforeResLen: 1,
			// Should take the optimized path where we only watch the service index,
			// connect index iterator, and gateway-services iterator.
			wantBeforeWatchSetSize: 3,
			updateFn: func(s *Store) {
				testRegisterCheck(t, s, 7, "node1", "", "check1", "critical")
			},
			shouldFire:      true,
			wantAfterIndex:  7,
			wantAfterResLen: 1, // critical filtering doesn't happen in the state store method.
			// Should take the optimized path where we only watch the service index,
			// connect index iterator, and gateway-services iterator.
			wantAfterWatchSetSize: 3,
		},
		{
			// See https://github.com/hashicorp/consul/issues/5506. The issue is cause
			// if the target service exists and is registered meaning it has a
			// service-specific index. This index is then used for the connect query
			// even though it is not updated by changes to the actual proxy or it's
			// checks. If the target service was never registered then it all appears
			// to work because the code would not find a service index and so fall
			// back to using the global service index which does change on any update
			// to proxies.
			name: "unblocks on proxy service health check change with target service present",
			setupFn: func(s *Store) {
				testRegisterService(t, s, 4, "node1", "test") // normal service
				testRegisterSidecarProxy(t, s, 5, "node1", "test")
				testRegisterCheck(t, s, 6, "node1", "test-sidecar-proxy", "check1", "passing")
			},
			svc:              "test",
			wantBeforeResLen: 1,
			// Should take the optimized path where we only watch the service index,
			// connect index iterator, and gateway-services iterator.
			wantBeforeWatchSetSize: 3,
			updateFn: func(s *Store) {
				testRegisterCheck(t, s, 7, "node1", "test-sidecar-proxy", "check1", "critical")
			},
			shouldFire:      true,
			wantAfterIndex:  7,
			wantAfterResLen: 1, // critical filtering doesn't happen in the state store method.
			// Should take the optimized path where we only watch the service index,
			// connect index iterator, and gateway-services iterator.
			wantAfterWatchSetSize: 3,
		},
		{
			// See https://github.com/hashicorp/consul/issues/5506. This is the edge
			// case that the simple solution wouldn't catch.
			name: "unblocks on different service name proxy-service registration when service is present",
			setupFn: func(s *Store) {
				testRegisterSidecarProxy(t, s, 4, "node1", "test")
			},
			svc:              "test",
			wantBeforeResLen: 1,
			// Should take the optimized path where we only watch the service index,
			// connect index iterator, and gateway-services iterator.
			wantBeforeWatchSetSize: 3,
			updateFn: func(s *Store) {
				// Register a new result with a different service name could be another
				// proxy with a different name, but a native instance works too.
				testRegisterConnectNativeService(t, s, 5, "node2", "test")
			},
			shouldFire:      true,
			wantAfterIndex:  5,
			wantAfterResLen: 2,
			// Should take the optimized path where we only watch the service indexes,
			// connect index iterator, and gateway-services iterator.
			wantAfterWatchSetSize: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStateStore(t)

			// Always create 3 nodes
			testRegisterNode(t, s, 1, "node1")
			testRegisterNode(t, s, 2, "node2")
			testRegisterNode(t, s, 3, "node3")

			// Setup
			if tt.setupFn != nil {
				tt.setupFn(s)
			}

			// Run the query
			ws := memdb.NewWatchSet()
			_, res, err := s.CheckConnectServiceNodes(ws, tt.svc, nil, "")
			require.NoError(t, err)
			require.Len(t, res, tt.wantBeforeResLen)
			require.Len(t, ws, tt.wantBeforeWatchSetSize)

			// Mutate the state store
			if tt.updateFn != nil {
				tt.updateFn(s)
			}

			fired := watchFired(ws)
			if tt.shouldFire {
				require.True(t, fired, "WatchSet should have fired")
			} else {
				require.False(t, fired, "WatchSet should not have fired")
			}

			// Re-query the same result. Should return the desired index and len
			ws = memdb.NewWatchSet()
			idx, res, err := s.CheckConnectServiceNodes(ws, tt.svc, nil, "")
			require.NoError(t, err)
			require.Len(t, res, tt.wantAfterResLen)
			require.Equal(t, tt.wantAfterIndex, idx)
			require.Len(t, ws, tt.wantAfterWatchSetSize)
		})
	}
}

func TestStateStore_CheckServiceNodes(t *testing.T) {
	s := testStateStore(t)

	// Querying with no matches gives an empty response
	ws := memdb.NewWatchSet()
	idx, res, err := s.CheckServiceNodes(ws, "service1", nil, "")
	if idx != 0 || res != nil || err != nil {
		t.Fatalf("expected (0, nil, nil), got: (%d, %#v, %#v)", idx, res, err)
	}

	// Register some nodes.
	testRegisterNode(t, s, 0, "node1")
	testRegisterNode(t, s, 1, "node2")

	// Register node-level checks. These should be the final result.
	testRegisterCheck(t, s, 2, "node1", "", "check1", api.HealthPassing)
	testRegisterCheck(t, s, 3, "node2", "", "check2", api.HealthPassing)

	// Register a service against the nodes.
	testRegisterService(t, s, 4, "node1", "service1")
	testRegisterService(t, s, 5, "node2", "service2")

	// Register checks against the services.
	testRegisterCheck(t, s, 6, "node1", "service1", "check3", api.HealthPassing)
	testRegisterCheck(t, s, 7, "node2", "service2", "check4", api.HealthPassing)

	// At this point all the changes should have fired the watch.
	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	// We ensure the idx for service2 has not been changed
	ensureServiceVersion(t, s, ws, "service2", 7, 1)

	// Query the state store for nodes and checks which have been registered
	// with a specific service.
	ws = memdb.NewWatchSet()
	ensureServiceVersion(t, s, ws, "service1", 6, 1)
	idx, results, err := s.CheckServiceNodes(ws, "service1", nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	// registered with ensureServiceVersion(t, s, ws, "service1", 6, 1)
	if idx != 6 {
		t.Fatalf("bad index: %d", idx)
	}

	// Make sure we get the expected result (service check + node check).
	if n := len(results); n != 1 {
		t.Fatalf("expected 1 result, got: %d", n)
	}
	csn := results[0]
	if csn.Node == nil || csn.Service == nil || len(csn.Checks) != 2 ||
		csn.Checks[0].ServiceID != "" || csn.Checks[0].CheckID != "check1" ||
		csn.Checks[1].ServiceID != "service1" || csn.Checks[1].CheckID != "check3" {
		t.Fatalf("bad output: %#v", csn)
	}

	// Node updates alter the returned index and fire the watch.
	testRegisterNodeWithChange(t, s, 8, "node1")
	if !watchFired(ws) {
		t.Fatalf("bad")
	}
	ws = memdb.NewWatchSet()
	idx, _, err = s.CheckServiceNodes(ws, "service1", nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	// service1 has been updated by node on idx 8
	if idx != 8 {
		t.Fatalf("bad index: %d", idx)
	}

	// Service updates alter the returned index and fire the watch.

	testRegisterServiceWithChange(t, s, 9, "node1", "service1", true)
	if !watchFired(ws) {
		t.Fatalf("bad")
	}
	ws = memdb.NewWatchSet()
	idx, _, err = s.CheckServiceNodes(ws, "service1", nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 9 {
		t.Fatalf("bad index: %d", idx)
	}

	// Check updates alter the returned index and fire the watch.
	testRegisterCheck(t, s, 10, "node1", "service1", "check1", api.HealthCritical)
	if !watchFired(ws) {
		t.Fatalf("bad")
	}
	ws = memdb.NewWatchSet()
	idx, _, err = s.CheckServiceNodes(ws, "service1", nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 10 {
		t.Fatalf("bad index: %d", idx)
	}

	// Registering some unrelated node + service should not fire the watch.
	testRegisterNode(t, s, 11, "nope")
	testRegisterService(t, s, 12, "nope", "nope")
	if watchFired(ws) {
		t.Fatalf("bad")
	}

	// Note that we can't overwhelm chan tracking any more since we optimized it
	// to only need to watch one chan in the happy path. The only path that does
	// bees to watch more stuff is where there are no service instances which also
	// means fewer than watchLimit chans too so effectively no way to trigger
	// Fallback watch any more.
}

func TestStateStore_CheckConnectServiceNodes(t *testing.T) {
	s := testStateStore(t)

	// Listing with no results returns an empty list.
	ws := memdb.NewWatchSet()
	idx, nodes, err := s.CheckConnectServiceNodes(ws, "db", nil, "")
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(0))
	assert.Len(t, nodes, 0)

	// Create some nodes and services.
	assert.Nil(t, s.EnsureNode(10, &structs.Node{Node: "foo", Address: "127.0.0.1"}))
	assert.Nil(t, s.EnsureNode(11, &structs.Node{Node: "bar", Address: "127.0.0.2"}))
	assert.Nil(t, s.EnsureService(12, "foo", &structs.NodeService{ID: "db", Service: "db", Tags: nil, Address: "", Port: 5000}))
	assert.Nil(t, s.EnsureService(13, "bar", &structs.NodeService{ID: "api", Service: "api", Tags: nil, Address: "", Port: 5000}))
	assert.Nil(t, s.EnsureService(14, "foo", &structs.NodeService{Kind: structs.ServiceKindConnectProxy, ID: "proxy", Service: "proxy", Proxy: structs.ConnectProxyConfig{DestinationServiceName: "db"}, Port: 8000}))
	assert.Nil(t, s.EnsureService(15, "bar", &structs.NodeService{Kind: structs.ServiceKindConnectProxy, ID: "proxy", Service: "proxy", Proxy: structs.ConnectProxyConfig{DestinationServiceName: "db"}, Port: 8000}))
	assert.Nil(t, s.EnsureService(16, "bar", &structs.NodeService{ID: "db2", Service: "db", Tags: []string{"replica"}, Address: "", Port: 8001}))
	assert.True(t, watchFired(ws))

	// Register node checks
	testRegisterCheck(t, s, 17, "foo", "", "check1", api.HealthPassing)
	testRegisterCheck(t, s, 18, "bar", "", "check2", api.HealthPassing)

	// Register checks against the services.
	testRegisterCheck(t, s, 19, "foo", "db", "check3", api.HealthPassing)
	testRegisterCheck(t, s, 20, "bar", "proxy", "check4", api.HealthPassing)

	// Read everything back.
	ws = memdb.NewWatchSet()
	idx, nodes, err = s.CheckConnectServiceNodes(ws, "db", nil, "")
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(20))
	assert.Len(t, nodes, 2)

	for _, n := range nodes {
		assert.Equal(t, structs.ServiceKindConnectProxy, n.Service.Kind)
		assert.Equal(t, "db", n.Service.Proxy.DestinationServiceName)
	}
}

func TestStateStore_CheckConnectServiceNodes_Gateways(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	s := testStateStore(t)

	// Listing with no results returns an empty list.
	ws := memdb.NewWatchSet()
	idx, nodes, err := s.CheckConnectServiceNodes(ws, "db", nil, "")
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(0))
	assert.Len(t, nodes, 0)

	// Create some nodes and services.
	assert.Nil(t, s.EnsureNode(10, &structs.Node{Node: "foo", Address: "127.0.0.1"}))
	assert.Nil(t, s.EnsureNode(11, &structs.Node{Node: "bar", Address: "127.0.0.2"}))

	// Typical services
	assert.Nil(t, s.EnsureService(12, "foo", &structs.NodeService{ID: "db", Service: "db", Tags: nil, Address: "", Port: 5000}))
	assert.Nil(t, s.EnsureService(13, "bar", &structs.NodeService{ID: "api", Service: "api", Tags: nil, Address: "", Port: 5000}))
	assert.Nil(t, s.EnsureService(14, "bar", &structs.NodeService{ID: "db2", Service: "db", Tags: []string{"replica"}, Address: "", Port: 8001}))
	assert.False(t, watchFired(ws))

	// Register node and service checks
	testRegisterCheck(t, s, 15, "foo", "", "check1", api.HealthPassing)
	testRegisterCheck(t, s, 16, "bar", "", "check2", api.HealthPassing)
	testRegisterCheck(t, s, 17, "foo", "db", "check3", api.HealthPassing)
	assert.False(t, watchFired(ws))

	// Watch should fire when a gateway is associated with the service, even if the gateway doesn't exist yet
	assert.Nil(t, s.EnsureConfigEntry(18, &structs.TerminatingGatewayConfigEntry{
		Kind: "terminating-gateway",
		Name: "gateway",
		Services: []structs.LinkedService{
			{
				Name: "db",
			},
		},
	}))
	assert.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	idx, nodes, err = s.CheckConnectServiceNodes(ws, "db", nil, "")
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(18))
	assert.Len(t, nodes, 0)

	// Watch should fire when a gateway is added
	assert.Nil(t, s.EnsureService(19, "bar", &structs.NodeService{Kind: structs.ServiceKindTerminatingGateway, ID: "gateway", Service: "gateway", Port: 443}))
	assert.True(t, watchFired(ws))

	// Watch should fire when a check is added to the gateway
	testRegisterCheck(t, s, 20, "bar", "gateway", "check4", api.HealthPassing)
	assert.True(t, watchFired(ws))

	// Watch should fire when a different connect service is registered for db
	assert.Nil(t, s.EnsureService(21, "foo", &structs.NodeService{Kind: structs.ServiceKindConnectProxy, ID: "proxy", Service: "proxy", Proxy: structs.ConnectProxyConfig{DestinationServiceName: "db"}, Port: 8000}))
	assert.True(t, watchFired(ws))

	// Read everything back.
	ws = memdb.NewWatchSet()
	idx, nodes, err = s.CheckConnectServiceNodes(ws, "db", nil, "")
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(21))
	assert.Len(t, nodes, 2)

	// Check sidecar
	assert.Equal(t, structs.ServiceKindConnectProxy, nodes[0].Service.Kind)
	assert.Equal(t, "foo", nodes[0].Node.Node)
	assert.Equal(t, "proxy", nodes[0].Service.Service)
	assert.Equal(t, "proxy", nodes[0].Service.ID)
	assert.Equal(t, "db", nodes[0].Service.Proxy.DestinationServiceName)
	assert.Equal(t, 8000, nodes[0].Service.Port)

	// Check gateway
	assert.Equal(t, structs.ServiceKindTerminatingGateway, nodes[1].Service.Kind)
	assert.Equal(t, "bar", nodes[1].Node.Node)
	assert.Equal(t, "gateway", nodes[1].Service.Service)
	assert.Equal(t, "gateway", nodes[1].Service.ID)
	assert.Equal(t, 443, nodes[1].Service.Port)

	// Watch should fire when another gateway instance is registered
	assert.Nil(t, s.EnsureService(22, "foo", &structs.NodeService{Kind: structs.ServiceKindTerminatingGateway, ID: "gateway-2", Service: "gateway", Port: 443}))
	assert.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	idx, nodes, err = s.CheckConnectServiceNodes(ws, "db", nil, "")
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(22))
	assert.Len(t, nodes, 3)

	// Watch should fire when a gateway instance is deregistered
	assert.Nil(t, s.DeleteService(23, "bar", "gateway", nil, ""))
	assert.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	idx, nodes, err = s.CheckConnectServiceNodes(ws, "db", nil, "")
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(23))
	assert.Len(t, nodes, 2)

	// Check new gateway
	assert.Equal(t, structs.ServiceKindTerminatingGateway, nodes[1].Service.Kind)
	assert.Equal(t, "foo", nodes[1].Node.Node)
	assert.Equal(t, "gateway", nodes[1].Service.Service)
	assert.Equal(t, "gateway-2", nodes[1].Service.ID)
	assert.Equal(t, 443, nodes[1].Service.Port)

	// Index should not slide back after deleting all instances of the gateway
	assert.Nil(t, s.DeleteService(24, "foo", "gateway-2", nil, ""))
	assert.True(t, watchFired(ws))

	idx, nodes, err = s.CheckConnectServiceNodes(ws, "db", nil, "")
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(24))
	assert.Len(t, nodes, 1)

	// Ensure that remaining node is the proxy and not a gateway
	assert.Equal(t, structs.ServiceKindConnectProxy, nodes[0].Service.Kind)
	assert.Equal(t, "foo", nodes[0].Node.Node)
	assert.Equal(t, "proxy", nodes[0].Service.Service)
	assert.Equal(t, "proxy", nodes[0].Service.ID)
	assert.Equal(t, 8000, nodes[0].Service.Port)
}

func BenchmarkCheckServiceNodes(b *testing.B) {
	s := NewStateStore(nil)

	if err := s.EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"}); err != nil {
		b.Fatalf("err: %v", err)
	}
	if err := s.EnsureService(2, "foo", &structs.NodeService{ID: "db1", Service: "db", Tags: []string{"primary"}, Address: "", Port: 8000}); err != nil {
		b.Fatalf("err: %v", err)
	}
	check := &structs.HealthCheck{
		Node:      "foo",
		CheckID:   "db",
		Name:      "can connect",
		Status:    api.HealthPassing,
		ServiceID: "db1",
	}
	if err := s.EnsureCheck(3, check); err != nil {
		b.Fatalf("err: %v", err)
	}
	check = &structs.HealthCheck{
		Node:    "foo",
		CheckID: "check1",
		Name:    "check1",
		Status:  api.HealthPassing,
	}
	if err := s.EnsureCheck(4, check); err != nil {
		b.Fatalf("err: %v", err)
	}

	ws := memdb.NewWatchSet()
	for i := 0; i < b.N; i++ {
		s.CheckServiceNodes(ws, "db", nil, "")
	}
}

func TestStateStore_CheckServiceTagNodes(t *testing.T) {
	s := testStateStore(t)

	if err := s.EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := s.EnsureService(2, "foo", &structs.NodeService{ID: "db1", Service: "db", Tags: []string{"primary"}, Address: "", Port: 8000}); err != nil {
		t.Fatalf("err: %v", err)
	}
	check := &structs.HealthCheck{
		Node:      "foo",
		CheckID:   "db",
		Name:      "can connect",
		Status:    api.HealthPassing,
		ServiceID: "db1",
	}
	if err := s.EnsureCheck(3, check); err != nil {
		t.Fatalf("err: %v", err)
	}
	check = &structs.HealthCheck{
		Node:    "foo",
		CheckID: "check1",
		Name:    "another check",
		Status:  api.HealthPassing,
	}
	if err := s.EnsureCheck(4, check); err != nil {
		t.Fatalf("err: %v", err)
	}

	ws := memdb.NewWatchSet()
	idx, nodes, err := s.CheckServiceTagNodes(ws, "db", []string{"primary"}, nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 4 {
		t.Fatalf("bad: %v", idx)
	}
	if len(nodes) != 1 {
		t.Fatalf("Bad: %v", nodes)
	}
	if nodes[0].Node.Node != "foo" {
		t.Fatalf("Bad: %v", nodes[0])
	}
	if nodes[0].Service.ID != "db1" {
		t.Fatalf("Bad: %v", nodes[0])
	}
	if len(nodes[0].Checks) != 2 {
		t.Fatalf("Bad: %v", nodes[0])
	}
	if nodes[0].Checks[0].CheckID != "check1" {
		t.Fatalf("Bad: %v", nodes[0])
	}
	if nodes[0].Checks[1].CheckID != "db" {
		t.Fatalf("Bad: %v", nodes[0])
	}

	// Changing a tag should fire the watch.
	if err := s.EnsureService(4, "foo", &structs.NodeService{ID: "db1", Service: "db", Tags: []string{"nope"}, Address: "", Port: 8000}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_Check_Snapshot(t *testing.T) {
	s := testStateStore(t)

	// Create a node, a service, and a service check as well as a node check.
	testRegisterNode(t, s, 0, "node1")
	testRegisterService(t, s, 1, "node1", "service1")
	checks := structs.HealthChecks{
		&structs.HealthCheck{
			Node:    "node1",
			CheckID: "check1",
			Name:    "node check",
			Status:  api.HealthPassing,
		},
		&structs.HealthCheck{
			Node:      "node1",
			CheckID:   "check2",
			Name:      "service check",
			Status:    api.HealthCritical,
			ServiceID: "service1",
		},
	}
	for i, hc := range checks {
		if err := s.EnsureCheck(uint64(i+1), hc); err != nil {
			t.Fatalf("err: %s", err)
		}
	}

	// Create a second node/service to make sure node filtering works. This
	// will affect the index but not the dump.
	testRegisterNode(t, s, 3, "node2")
	testRegisterService(t, s, 4, "node2", "service2")
	testRegisterCheck(t, s, 5, "node2", "service2", "check3", api.HealthPassing)

	// Snapshot the checks.
	snap := s.Snapshot()
	defer snap.Close()

	// Alter the real state store.
	testRegisterCheck(t, s, 6, "node2", "service2", "check4", api.HealthPassing)

	// Verify the snapshot.
	if idx := snap.LastIndex(); idx != 5 {
		t.Fatalf("bad index: %d", idx)
	}
	iter, err := snap.Checks("node1", nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	for i := 0; i < len(checks); i++ {
		check := iter.Next().(*structs.HealthCheck)
		if check == nil {
			t.Fatalf("unexpected end of checks")
		}

		checks[i].CreateIndex, checks[i].ModifyIndex = uint64(i+1), uint64(i+1)
		if !reflect.DeepEqual(check, checks[i]) {
			t.Fatalf("bad: %#v != %#v", check, checks[i])
		}
	}
	if iter.Next() != nil {
		t.Fatalf("unexpected extra checks")
	}
}

func TestStateStore_ServiceDump(t *testing.T) {
	s := testStateStore(t)

	type operation struct {
		name      string
		modFn     func(*testing.T)
		allFired  bool
		kindFired bool
		checkAll  func(*testing.T, structs.CheckServiceNodes)
		checkKind func(*testing.T, structs.CheckServiceNodes)
	}

	sortDump := func(dump structs.CheckServiceNodes) {
		sort.Slice(dump, func(i, j int) bool {
			if dump[i].Node.Node < dump[j].Node.Node {
				return true
			} else if dump[i].Node.Node > dump[j].Node.Node {
				return false
			}

			if dump[i].Service.Service < dump[j].Service.Service {
				return true
			} else if dump[i].Service.Service > dump[j].Service.Service {
				return false
			}

			return false
		})

		for i := 0; i < len(dump); i++ {
			sort.Slice(dump[i].Checks, func(m, n int) bool {
				return dump[i].Checks[m].CheckID < dump[i].Checks[n].CheckID
			})
		}
	}

	operations := []operation{
		{
			name: "register some nodes",
			modFn: func(t *testing.T) {
				testRegisterNode(t, s, 0, "node1")
				testRegisterNode(t, s, 1, "node2")
			},
			allFired:  true, // fires due to "index"
			kindFired: true, // fires due to "index"
			checkAll: func(t *testing.T, dump structs.CheckServiceNodes) {
				require.Len(t, dump, 0)
			},
			checkKind: func(t *testing.T, dump structs.CheckServiceNodes) {
				require.Len(t, dump, 0)
			},
		},
		{
			name: "register services against them",
			modFn: func(t *testing.T) {
				testRegisterService(t, s, 2, "node1", "service1")
				testRegisterSidecarProxy(t, s, 3, "node1", "service1")
				testRegisterService(t, s, 4, "node2", "service1")
				testRegisterSidecarProxy(t, s, 5, "node2", "service1")
			},
			allFired:  true, // fires due to "index"
			kindFired: true, // fires due to "index"
			checkAll: func(t *testing.T, dump structs.CheckServiceNodes) {
				require.Len(t, dump, 4)
				require.Equal(t, "node1", dump[0].Node.Node)
				require.Equal(t, "node1", dump[1].Node.Node)
				require.Equal(t, "node2", dump[2].Node.Node)
				require.Equal(t, "node2", dump[3].Node.Node)

				require.Equal(t, "service1", dump[0].Service.Service)
				require.Equal(t, "service1-sidecar-proxy", dump[1].Service.Service)
				require.Equal(t, "service1", dump[2].Service.Service)
				require.Equal(t, "service1-sidecar-proxy", dump[3].Service.Service)

				require.Len(t, dump[0].Checks, 0)
				require.Len(t, dump[1].Checks, 0)
				require.Len(t, dump[2].Checks, 0)
				require.Len(t, dump[3].Checks, 0)
			},
			checkKind: func(t *testing.T, dump structs.CheckServiceNodes) {
				require.Len(t, dump, 2)

				require.Equal(t, "node1", dump[0].Node.Node)
				require.Equal(t, "node2", dump[1].Node.Node)

				require.Equal(t, "service1-sidecar-proxy", dump[0].Service.Service)
				require.Equal(t, "service1-sidecar-proxy", dump[1].Service.Service)

				require.Len(t, dump[0].Checks, 0)
				require.Len(t, dump[1].Checks, 0)
			},
		},
		{
			name: "register service-level checks",
			modFn: func(t *testing.T) {
				testRegisterCheck(t, s, 6, "node1", "service1", "check1", api.HealthCritical)
				testRegisterCheck(t, s, 7, "node2", "service1-sidecar-proxy", "check1", api.HealthCritical)
			},
			allFired:  true, // fires due to "index"
			kindFired: true, // fires due to "index"
			checkAll: func(t *testing.T, dump structs.CheckServiceNodes) {
				require.Len(t, dump, 4)
				require.Equal(t, "node1", dump[0].Node.Node)
				require.Equal(t, "node1", dump[1].Node.Node)
				require.Equal(t, "node2", dump[2].Node.Node)
				require.Equal(t, "node2", dump[3].Node.Node)

				require.Equal(t, "service1", dump[0].Service.Service)
				require.Equal(t, "service1-sidecar-proxy", dump[1].Service.Service)
				require.Equal(t, "service1", dump[2].Service.Service)
				require.Equal(t, "service1-sidecar-proxy", dump[3].Service.Service)

				require.Len(t, dump[0].Checks, 1)
				require.Len(t, dump[1].Checks, 0)
				require.Len(t, dump[2].Checks, 0)
				require.Len(t, dump[3].Checks, 1)

				require.Equal(t, api.HealthCritical, dump[0].Checks[0].Status)
				require.Equal(t, api.HealthCritical, dump[3].Checks[0].Status)
			},
			checkKind: func(t *testing.T, dump structs.CheckServiceNodes) {
				require.Len(t, dump, 2)

				require.Equal(t, "node1", dump[0].Node.Node)
				require.Equal(t, "node2", dump[1].Node.Node)

				require.Equal(t, "service1-sidecar-proxy", dump[0].Service.Service)
				require.Equal(t, "service1-sidecar-proxy", dump[1].Service.Service)

				require.Len(t, dump[0].Checks, 0)
				require.Len(t, dump[1].Checks, 1)

				require.Equal(t, api.HealthCritical, dump[1].Checks[0].Status)
			},
		},
		{
			name: "register node-level checks",
			modFn: func(t *testing.T) {
				testRegisterCheck(t, s, 8, "node1", "", "check2", api.HealthPassing)
				testRegisterCheck(t, s, 9, "node2", "", "check2", api.HealthPassing)
			},
			allFired:  true, // fires due to "index"
			kindFired: true, // fires due to "index"
			checkAll: func(t *testing.T, dump structs.CheckServiceNodes) {
				require.Len(t, dump, 4)
				require.Equal(t, "node1", dump[0].Node.Node)
				require.Equal(t, "node1", dump[1].Node.Node)
				require.Equal(t, "node2", dump[2].Node.Node)
				require.Equal(t, "node2", dump[3].Node.Node)

				require.Equal(t, "service1", dump[0].Service.Service)
				require.Equal(t, "service1-sidecar-proxy", dump[1].Service.Service)
				require.Equal(t, "service1", dump[2].Service.Service)
				require.Equal(t, "service1-sidecar-proxy", dump[3].Service.Service)

				require.Len(t, dump[0].Checks, 2)
				require.Len(t, dump[1].Checks, 1)
				require.Len(t, dump[2].Checks, 1)
				require.Len(t, dump[3].Checks, 2)

				require.Equal(t, api.HealthCritical, dump[0].Checks[0].Status)
				require.Equal(t, api.HealthPassing, dump[0].Checks[1].Status)
				require.Equal(t, api.HealthPassing, dump[1].Checks[0].Status)
				require.Equal(t, api.HealthPassing, dump[2].Checks[0].Status)
				require.Equal(t, api.HealthCritical, dump[3].Checks[0].Status)
				require.Equal(t, api.HealthPassing, dump[3].Checks[1].Status)
			},
			checkKind: func(t *testing.T, dump structs.CheckServiceNodes) {
				require.Len(t, dump, 2)

				require.Equal(t, "node1", dump[0].Node.Node)
				require.Equal(t, "node2", dump[1].Node.Node)

				require.Equal(t, "service1-sidecar-proxy", dump[0].Service.Service)
				require.Equal(t, "service1-sidecar-proxy", dump[1].Service.Service)

				require.Len(t, dump[0].Checks, 1)
				require.Len(t, dump[1].Checks, 2)

				require.Equal(t, api.HealthPassing, dump[0].Checks[0].Status)
				require.Equal(t, api.HealthCritical, dump[1].Checks[0].Status)
				require.Equal(t, api.HealthPassing, dump[1].Checks[1].Status)
			},
		},
		{
			name: "pass a previously failing check",
			modFn: func(t *testing.T) {
				testRegisterCheck(t, s, 10, "node1", "service1", "check1", api.HealthPassing)
				testRegisterCheck(t, s, 11, "node2", "service1-sidecar-proxy", "check1", api.HealthPassing)
			},
			allFired:  true, // fires due to "index"
			kindFired: true, // fires due to "index"
			checkAll: func(t *testing.T, dump structs.CheckServiceNodes) {
				require.Len(t, dump, 4)
				require.Equal(t, "node1", dump[0].Node.Node)
				require.Equal(t, "node1", dump[1].Node.Node)
				require.Equal(t, "node2", dump[2].Node.Node)
				require.Equal(t, "node2", dump[3].Node.Node)

				require.Equal(t, "service1", dump[0].Service.Service)
				require.Equal(t, "service1-sidecar-proxy", dump[1].Service.Service)
				require.Equal(t, "service1", dump[2].Service.Service)
				require.Equal(t, "service1-sidecar-proxy", dump[3].Service.Service)

				require.Len(t, dump[0].Checks, 2)
				require.Len(t, dump[1].Checks, 1)
				require.Len(t, dump[2].Checks, 1)
				require.Len(t, dump[3].Checks, 2)

				require.Equal(t, api.HealthPassing, dump[0].Checks[0].Status)
				require.Equal(t, api.HealthPassing, dump[0].Checks[1].Status)
				require.Equal(t, api.HealthPassing, dump[1].Checks[0].Status)
				require.Equal(t, api.HealthPassing, dump[2].Checks[0].Status)
				require.Equal(t, api.HealthPassing, dump[3].Checks[0].Status)
				require.Equal(t, api.HealthPassing, dump[3].Checks[1].Status)
			},
			checkKind: func(t *testing.T, dump structs.CheckServiceNodes) {
				require.Len(t, dump, 2)

				require.Equal(t, "node1", dump[0].Node.Node)
				require.Equal(t, "node2", dump[1].Node.Node)

				require.Equal(t, "service1-sidecar-proxy", dump[0].Service.Service)
				require.Equal(t, "service1-sidecar-proxy", dump[1].Service.Service)

				require.Len(t, dump[0].Checks, 1)
				require.Len(t, dump[1].Checks, 2)

				require.Equal(t, api.HealthPassing, dump[0].Checks[0].Status)
				require.Equal(t, api.HealthPassing, dump[1].Checks[0].Status)
				require.Equal(t, api.HealthPassing, dump[1].Checks[1].Status)
			},
		},
		{
			name: "delete a node",
			modFn: func(t *testing.T) {
				s.DeleteNode(12, "node2", nil, "")
			},
			allFired:  true, // fires due to "index"
			kindFired: true, // fires due to "index"
			checkAll: func(t *testing.T, dump structs.CheckServiceNodes) {
				require.Len(t, dump, 2)
				require.Equal(t, "node1", dump[0].Node.Node)
				require.Equal(t, "node1", dump[1].Node.Node)

				require.Equal(t, "service1", dump[0].Service.Service)
				require.Equal(t, "service1-sidecar-proxy", dump[1].Service.Service)

				require.Len(t, dump[0].Checks, 2)
				require.Len(t, dump[1].Checks, 1)

				require.Equal(t, api.HealthPassing, dump[0].Checks[0].Status)
				require.Equal(t, api.HealthPassing, dump[0].Checks[1].Status)
				require.Equal(t, api.HealthPassing, dump[1].Checks[0].Status)
			},
			checkKind: func(t *testing.T, dump structs.CheckServiceNodes) {
				require.Len(t, dump, 1)

				require.Equal(t, "node1", dump[0].Node.Node)

				require.Equal(t, "service1-sidecar-proxy", dump[0].Service.Service)

				require.Len(t, dump[0].Checks, 1)

				require.Equal(t, api.HealthPassing, dump[0].Checks[0].Status)
			},
		},
	}
	for _, op := range operations {
		op := op
		require.True(t, t.Run(op.name, func(t *testing.T) {
			wsAll := memdb.NewWatchSet()
			_, _, err := s.ServiceDump(wsAll, "", false, nil, "")
			require.NoError(t, err)

			wsKind := memdb.NewWatchSet()
			_, _, err = s.ServiceDump(wsKind, structs.ServiceKindConnectProxy, true, nil, "")
			require.NoError(t, err)

			op.modFn(t)

			require.Equal(t, op.allFired, watchFired(wsAll), "all dump watch firing busted")
			require.Equal(t, op.kindFired, watchFired(wsKind), "kind dump watch firing busted")

			_, dump, err := s.ServiceDump(nil, "", false, nil, "")
			require.NoError(t, err)
			sortDump(dump)
			op.checkAll(t, dump)

			_, dump, err = s.ServiceDump(nil, structs.ServiceKindConnectProxy, true, nil, "")
			require.NoError(t, err)
			sortDump(dump)
			op.checkKind(t, dump)
		}))
	}
}

func TestStateStore_NodeInfo_NodeDump(t *testing.T) {
	s := testStateStore(t)

	// Generating a node dump that matches nothing returns empty
	wsInfo := memdb.NewWatchSet()
	idx, dump, err := s.NodeInfo(wsInfo, "node1", nil, "")
	if idx != 0 || dump != nil || err != nil {
		t.Fatalf("expected (0, nil, nil), got: (%d, %#v, %#v)", idx, dump, err)
	}
	wsDump := memdb.NewWatchSet()
	idx, dump, err = s.NodeDump(wsDump, nil, "")
	if idx != 0 || dump != nil || err != nil {
		t.Fatalf("expected (0, nil, nil), got: (%d, %#v, %#v)", idx, dump, err)
	}

	// Register some nodes
	// node1 is registered withOut any nodemeta, and a consul service with id
	// 'consul' is added later with meta 'version'. The expected node must have
	// meta 'consul-version' with same value
	testRegisterNode(t, s, 0, "node1")
	testRegisterNode(t, s, 1, "node2")

	// Register services against them
	testRegisterService(t, s, 2, "node1", "service1")
	testRegisterService(t, s, 3, "node1", "service2")
	testRegisterService(t, s, 4, "node2", "service1")
	testRegisterService(t, s, 5, "node2", "service2")
	// Register consul service with meta 'version' for node1
	testRegisterServiceWithMeta(t, s, 10, "node1", "consul", map[string]string{"version": "1.17.0"})

	// Register service-level checks
	testRegisterCheck(t, s, 6, "node1", "service1", "check1", api.HealthPassing)
	testRegisterCheck(t, s, 7, "node2", "service1", "check1", api.HealthPassing)

	// Register node-level checks
	testRegisterCheck(t, s, 8, "node1", "", "check2", api.HealthPassing)
	testRegisterCheck(t, s, 9, "node2", "", "check2", api.HealthPassing)

	// Both watches should have fired due to the changes above.
	if !watchFired(wsInfo) {
		t.Fatalf("bad")
	}
	if !watchFired(wsDump) {
		t.Fatalf("bad")
	}

	// Check that our result matches what we expect.
	expect := structs.NodeDump{
		&structs.NodeInfo{
			Node:      "node1",
			Partition: structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:        "node1",
					CheckID:     "check1",
					ServiceID:   "service1",
					ServiceName: "service1",
					Status:      api.HealthPassing,
					RaftIndex: structs.RaftIndex{
						CreateIndex: 6,
						ModifyIndex: 6,
					},
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				},
				&structs.HealthCheck{
					Node:        "node1",
					CheckID:     "check2",
					ServiceID:   "",
					ServiceName: "",
					Status:      api.HealthPassing,
					RaftIndex: structs.RaftIndex{
						CreateIndex: 8,
						ModifyIndex: 8,
					},
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				},
			},
			Services: []*structs.NodeService{
				{
					ID:      "consul",
					Service: "consul",
					Address: "1.1.1.1",
					Meta:    map[string]string{"version": "1.17.0"},
					Port:    1111,
					Weights: &structs.Weights{Passing: 1, Warning: 1},
					RaftIndex: structs.RaftIndex{
						CreateIndex: 10,
						ModifyIndex: 10,
					},
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				},
				{
					ID:      "service1",
					Service: "service1",
					Address: "1.1.1.1",
					Meta:    make(map[string]string),
					Port:    1111,
					Weights: &structs.Weights{Passing: 1, Warning: 1},
					RaftIndex: structs.RaftIndex{
						CreateIndex: 2,
						ModifyIndex: 2,
					},
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				},
				{
					ID:      "service2",
					Service: "service2",
					Address: "1.1.1.1",
					Meta:    make(map[string]string),
					Port:    1111,
					Weights: &structs.Weights{Passing: 1, Warning: 1},
					RaftIndex: structs.RaftIndex{
						CreateIndex: 3,
						ModifyIndex: 3,
					},
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				},
			},
			Meta: map[string]string{"consul-version": "1.17.0"},
		},
		&structs.NodeInfo{
			Node:      "node2",
			Partition: structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:        "node2",
					CheckID:     "check1",
					ServiceID:   "service1",
					ServiceName: "service1",
					Status:      api.HealthPassing,
					RaftIndex: structs.RaftIndex{
						CreateIndex: 7,
						ModifyIndex: 7,
					},
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				},
				&structs.HealthCheck{
					Node:        "node2",
					CheckID:     "check2",
					ServiceID:   "",
					ServiceName: "",
					Status:      api.HealthPassing,
					RaftIndex: structs.RaftIndex{
						CreateIndex: 9,
						ModifyIndex: 9,
					},
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				},
			},
			Services: []*structs.NodeService{
				{
					ID:      "service1",
					Service: "service1",
					Address: "1.1.1.1",
					Port:    1111,
					Meta:    make(map[string]string),
					Weights: &structs.Weights{Passing: 1, Warning: 1},
					RaftIndex: structs.RaftIndex{
						CreateIndex: 4,
						ModifyIndex: 4,
					},
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				},
				{
					ID:      "service2",
					Service: "service2",
					Address: "1.1.1.1",
					Port:    1111,
					Meta:    make(map[string]string),
					Weights: &structs.Weights{Passing: 1, Warning: 1},
					RaftIndex: structs.RaftIndex{
						CreateIndex: 5,
						ModifyIndex: 5,
					},
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				},
			},
		},
	}

	// Get a dump of just a single node
	ws := memdb.NewWatchSet()
	idx, dump, err = s.NodeInfo(ws, "node1", nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 10 {
		t.Fatalf("bad index: %d", idx)
	}
	require.Len(t, dump, 1)
	require.Equal(t, expect[0], dump[0])

	// Generate a dump of all the nodes
	idx, dump, err = s.NodeDump(nil, nil, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 10 {
		t.Fatalf("bad index: %d", idx)
	}
	if !reflect.DeepEqual(dump, expect) {
		t.Fatalf("bad: %#v", dump[0].Services[0])
	}

	// Registering some unrelated node + service + check should not fire the
	// watch.
	testRegisterNode(t, s, 10, "nope")
	testRegisterService(t, s, 11, "nope", "nope")
	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_ServiceIdxUpdateOnNodeUpdate(t *testing.T) {
	s := testStateStore(t)

	// Create a service on a node
	err := s.EnsureNode(10, &structs.Node{Node: "node", Address: "127.0.0.1"})
	require.Nil(t, err)
	err = s.EnsureService(12, "node", &structs.NodeService{ID: "srv", Service: "srv", Tags: nil, Address: "", Port: 5000})
	require.Nil(t, err)

	// Store the current service index
	ws := memdb.NewWatchSet()
	lastIdx, _, err := s.ServiceNodes(ws, "srv", nil, "")
	require.Nil(t, err)

	// Update the node with some meta
	err = s.EnsureNode(14, &structs.Node{Node: "node", Address: "127.0.0.1", Meta: map[string]string{"foo": "bar"}})
	require.Nil(t, err)

	// Read the new service index
	ws = memdb.NewWatchSet()
	newIdx, _, err := s.ServiceNodes(ws, "srv", nil, "")
	require.Nil(t, err)

	require.True(t, newIdx > lastIdx)
}

func TestStateStore_ensureServiceCASTxn(t *testing.T) {
	s := testStateStore(t)

	testRegisterNode(t, s, 1, "node1")

	// Register a service
	testRegisterService(t, s, 2, "node1", "foo")

	ns := structs.NodeService{
		ID:      "foo",
		Service: "foo",
		// the testRegisterServices registers it with 111 as a port
		RaftIndex: structs.RaftIndex{
			ModifyIndex: 0,
		},
	}

	// attempt to update with a 0 index
	tx := s.db.WriteTxnRestore()
	err := ensureServiceCASTxn(tx, 3, "node1", &ns)
	require.Equal(t, err, errCASCompareFailed)
	require.NoError(t, tx.Commit())

	// ensure no update happened
	roTxn := s.db.Txn(false)
	_, nsRead, err := s.NodeService(nil, "node1", "foo", nil, "")
	require.NoError(t, err)
	require.NotNil(t, nsRead)
	require.Equal(t, uint64(2), nsRead.ModifyIndex)
	roTxn.Commit()

	ns.ModifyIndex = 99
	// attempt to update with a non-matching index
	tx = s.db.WriteTxnRestore()
	err = ensureServiceCASTxn(tx, 4, "node1", &ns)
	require.Equal(t, err, errCASCompareFailed)
	require.NoError(t, tx.Commit())

	// ensure no update happened
	roTxn = s.db.Txn(false)
	_, nsRead, err = s.NodeService(nil, "node1", "foo", nil, "")
	require.NoError(t, err)
	require.NotNil(t, nsRead)
	require.Equal(t, uint64(2), nsRead.ModifyIndex)
	roTxn.Commit()

	ns.ModifyIndex = 2
	// update with the matching modify index
	tx = s.db.WriteTxnRestore()
	err = ensureServiceCASTxn(tx, 7, "node1", &ns)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	// ensure the update happened
	roTxn = s.db.Txn(false)
	_, nsRead, err = s.NodeService(nil, "node1", "foo", nil, "")
	require.NoError(t, err)
	require.NotNil(t, nsRead)
	require.Equal(t, uint64(7), nsRead.ModifyIndex)
	roTxn.Commit()
}

func TestStateStore_GatewayServices_Terminating(t *testing.T) {
	s := testStateStore(t)

	// Listing with no results returns an empty list.
	ws := memdb.NewWatchSet()
	idx, nodes, err := s.GatewayServices(ws, "db", nil)
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(0))
	assert.Len(t, nodes, 0)

	// Create some nodes
	assert.Nil(t, s.EnsureNode(10, &structs.Node{Node: "foo", Address: "127.0.0.1"}))
	assert.Nil(t, s.EnsureNode(11, &structs.Node{Node: "bar", Address: "127.0.0.2"}))
	assert.Nil(t, s.EnsureNode(12, &structs.Node{Node: "baz", Address: "127.0.0.2"}))

	// Typical services and some consul services spread across two nodes
	assert.Nil(t, s.EnsureService(13, "foo", &structs.NodeService{ID: "db", Service: "db", Tags: nil, Address: "", Port: 5000}))
	assert.Nil(t, s.EnsureService(15, "bar", &structs.NodeService{ID: "api", Service: "api", Tags: nil, Address: "", Port: 5000}))
	assert.Nil(t, s.EnsureService(16, "bar", &structs.NodeService{ID: "consul", Service: "consul", Tags: nil}))
	assert.Nil(t, s.EnsureService(17, "bar", &structs.NodeService{ID: "consul", Service: "consul", Tags: nil}))

	// Add ingress gateway and a connect proxy, neither should get picked up by terminating gateway
	ingressNS := &structs.NodeService{
		Kind:    structs.ServiceKindIngressGateway,
		ID:      "ingress",
		Service: "ingress",
		Port:    8443,
	}
	assert.Nil(t, s.EnsureService(18, "baz", ingressNS))

	proxyNS := &structs.NodeService{
		Kind:    structs.ServiceKindConnectProxy,
		ID:      "db proxy",
		Service: "db proxy",
		Proxy: structs.ConnectProxyConfig{
			DestinationServiceName: "db",
		},
		Port: 8000,
	}
	assert.Nil(t, s.EnsureService(19, "foo", proxyNS))

	// Register a gateway
	assert.Nil(t, s.EnsureService(20, "baz", &structs.NodeService{Kind: structs.ServiceKindTerminatingGateway, ID: "gateway", Service: "gateway", Port: 443}))

	// Associate gateway with db and api
	assert.Nil(t, s.EnsureConfigEntry(21, &structs.TerminatingGatewayConfigEntry{
		Kind: "terminating-gateway",
		Name: "gateway",
		Services: []structs.LinkedService{
			{
				Name: "db",
			},
			{
				Name: "api",
			},
		},
	}))
	assert.True(t, watchFired(ws))

	// Read everything back.
	ws = memdb.NewWatchSet()
	idx, out, err := s.GatewayServices(ws, "gateway", nil)
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(21))
	assert.Len(t, out, 2)

	expect := structs.GatewayServices{
		{
			Service:     structs.NewServiceName("api", nil),
			Gateway:     structs.NewServiceName("gateway", nil),
			GatewayKind: structs.ServiceKindTerminatingGateway,
			RaftIndex: structs.RaftIndex{
				CreateIndex: 21,
				ModifyIndex: 21,
			},
			ServiceKind: structs.GatewayServiceKindService,
		},
		{
			Service:     structs.NewServiceName("db", nil),
			Gateway:     structs.NewServiceName("gateway", nil),
			GatewayKind: structs.ServiceKindTerminatingGateway,
			RaftIndex: structs.RaftIndex{
				CreateIndex: 21,
				ModifyIndex: 21,
			},
			ServiceKind: structs.GatewayServiceKindService,
		},
	}
	assert.Equal(t, expect, out)

	// Check that we don't update on same exact config
	assert.Nil(t, s.EnsureConfigEntry(21, &structs.TerminatingGatewayConfigEntry{
		Kind: "terminating-gateway",
		Name: "gateway",
		Services: []structs.LinkedService{
			{
				Name: "db",
			},
			{
				Name: "api",
			},
		},
	}))
	assert.False(t, watchFired(ws))

	idx, out, err = s.GatewayServices(ws, "gateway", nil)
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(21))
	assert.Len(t, out, 2)

	expect = structs.GatewayServices{
		{
			Service:     structs.NewServiceName("api", nil),
			Gateway:     structs.NewServiceName("gateway", nil),
			GatewayKind: structs.ServiceKindTerminatingGateway,
			RaftIndex: structs.RaftIndex{
				CreateIndex: 21,
				ModifyIndex: 21,
			},
			ServiceKind: structs.GatewayServiceKindService,
		},
		{
			Service:     structs.NewServiceName("db", nil),
			Gateway:     structs.NewServiceName("gateway", nil),
			GatewayKind: structs.ServiceKindTerminatingGateway,
			RaftIndex: structs.RaftIndex{
				CreateIndex: 21,
				ModifyIndex: 21,
			},
			ServiceKind: structs.GatewayServiceKindService,
		},
	}
	assert.Equal(t, expect, out)

	// Associate gateway with a wildcard and add TLS config
	assert.Nil(t, s.EnsureConfigEntry(22, &structs.TerminatingGatewayConfigEntry{
		Kind: "terminating-gateway",
		Name: "gateway",
		Services: []structs.LinkedService{
			{
				Name:     "api",
				CAFile:   "api/ca.crt",
				CertFile: "api/client.crt",
				KeyFile:  "api/client.key",
				SNI:      "my-domain",
			},
			{
				Name: "db",
			},
			{
				Name:     "*",
				CAFile:   "ca.crt",
				CertFile: "client.crt",
				KeyFile:  "client.key",
				SNI:      "my-alt-domain",
			},
		},
	}))
	assert.True(t, watchFired(ws))

	// Read everything back.
	ws = memdb.NewWatchSet()
	idx, out, err = s.GatewayServices(ws, "gateway", nil)
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(22))
	assert.Len(t, out, 2)

	expect = structs.GatewayServices{
		{
			Service:     structs.NewServiceName("api", nil),
			Gateway:     structs.NewServiceName("gateway", nil),
			GatewayKind: structs.ServiceKindTerminatingGateway,
			CAFile:      "api/ca.crt",
			CertFile:    "api/client.crt",
			KeyFile:     "api/client.key",
			SNI:         "my-domain",
			RaftIndex: structs.RaftIndex{
				CreateIndex: 22,
				ModifyIndex: 22,
			},
			ServiceKind: structs.GatewayServiceKindService,
		},
		{
			Service:     structs.NewServiceName("db", nil),
			Gateway:     structs.NewServiceName("gateway", nil),
			GatewayKind: structs.ServiceKindTerminatingGateway,
			RaftIndex: structs.RaftIndex{
				CreateIndex: 22,
				ModifyIndex: 22,
			},
			ServiceKind: structs.GatewayServiceKindService,
		},
	}
	assert.Equal(t, expect, out)

	// Add a service covered by wildcard
	assert.Nil(t, s.EnsureService(23, "bar", &structs.NodeService{ID: "redis", Service: "redis", Tags: nil, Address: "", Port: 6379}))
	assert.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	idx, out, err = s.GatewayServices(ws, "gateway", nil)
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(23))
	assert.Len(t, out, 3)

	expect = structs.GatewayServices{
		{
			Service:     structs.NewServiceName("api", nil),
			Gateway:     structs.NewServiceName("gateway", nil),
			GatewayKind: structs.ServiceKindTerminatingGateway,
			CAFile:      "api/ca.crt",
			CertFile:    "api/client.crt",
			KeyFile:     "api/client.key",
			SNI:         "my-domain",
			RaftIndex: structs.RaftIndex{
				CreateIndex: 22,
				ModifyIndex: 22,
			},
			ServiceKind: structs.GatewayServiceKindService,
		},
		{
			Service:     structs.NewServiceName("db", nil),
			Gateway:     structs.NewServiceName("gateway", nil),
			GatewayKind: structs.ServiceKindTerminatingGateway,
			RaftIndex: structs.RaftIndex{
				CreateIndex: 22,
				ModifyIndex: 22,
			},
			ServiceKind: structs.GatewayServiceKindService,
		},
		{
			Service:      structs.NewServiceName("redis", nil),
			Gateway:      structs.NewServiceName("gateway", nil),
			GatewayKind:  structs.ServiceKindTerminatingGateway,
			CAFile:       "ca.crt",
			CertFile:     "client.crt",
			KeyFile:      "client.key",
			SNI:          "my-alt-domain",
			FromWildcard: true,
			RaftIndex: structs.RaftIndex{
				CreateIndex: 23,
				ModifyIndex: 23,
			},
			ServiceKind: structs.GatewayServiceKindService,
		},
	}
	assert.Equal(t, expect, out)

	// Delete a service covered by wildcard
	assert.Nil(t, s.DeleteService(24, "bar", "redis", nil, ""))
	assert.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	idx, out, err = s.GatewayServices(ws, "gateway", nil)
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(24))
	assert.Len(t, out, 2)

	expect = structs.GatewayServices{
		{
			Service:     structs.NewServiceName("api", nil),
			Gateway:     structs.NewServiceName("gateway", nil),
			GatewayKind: structs.ServiceKindTerminatingGateway,
			CAFile:      "api/ca.crt",
			CertFile:    "api/client.crt",
			KeyFile:     "api/client.key",
			SNI:         "my-domain",
			RaftIndex: structs.RaftIndex{
				CreateIndex: 22,
				ModifyIndex: 22,
			},
			ServiceKind: structs.GatewayServiceKindService,
		},
		{
			Service:     structs.NewServiceName("db", nil),
			Gateway:     structs.NewServiceName("gateway", nil),
			GatewayKind: structs.ServiceKindTerminatingGateway,
			RaftIndex: structs.RaftIndex{
				CreateIndex: 22,
				ModifyIndex: 22,
			},
			ServiceKind: structs.GatewayServiceKindService,
		},
	}
	assert.Equal(t, expect, out)

	// Update the entry that only leaves one service
	assert.Nil(t, s.EnsureConfigEntry(25, &structs.TerminatingGatewayConfigEntry{
		Kind: "terminating-gateway",
		Name: "gateway",
		Services: []structs.LinkedService{
			{
				Name: "db",
			},
		},
	}))
	assert.True(t, watchFired(ws))

	idx, out, err = s.GatewayServices(ws, "gateway", nil)
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(25))
	assert.Len(t, out, 1)

	// previously associated services should not be present
	expect = structs.GatewayServices{
		{
			Service:     structs.NewServiceName("db", nil),
			Gateway:     structs.NewServiceName("gateway", nil),
			GatewayKind: structs.ServiceKindTerminatingGateway,
			RaftIndex: structs.RaftIndex{
				CreateIndex: 25,
				ModifyIndex: 25,
			},
			ServiceKind: structs.GatewayServiceKindService,
		},
	}
	assert.Equal(t, expect, out)

	// Attempt to associate a different gateway with services that include db
	assert.Nil(t, s.EnsureConfigEntry(26, &structs.TerminatingGatewayConfigEntry{
		Kind: "terminating-gateway",
		Name: "gateway2",
		Services: []structs.LinkedService{
			{
				Name: "*",
			},
		},
	}))

	ws = memdb.NewWatchSet()
	idx, out, err = s.GatewayServices(ws, "gateway2", nil)
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(26))
	assert.Len(t, out, 2)

	expect = structs.GatewayServices{
		{
			Service:      structs.NewServiceName("api", nil),
			Gateway:      structs.NewServiceName("gateway2", nil),
			GatewayKind:  structs.ServiceKindTerminatingGateway,
			FromWildcard: true,
			RaftIndex: structs.RaftIndex{
				CreateIndex: 26,
				ModifyIndex: 26,
			},
		},
		{
			Service:      structs.NewServiceName("db", nil),
			Gateway:      structs.NewServiceName("gateway2", nil),
			GatewayKind:  structs.ServiceKindTerminatingGateway,
			FromWildcard: true,
			RaftIndex: structs.RaftIndex{
				CreateIndex: 26,
				ModifyIndex: 26,
			},
		},
	}
	assert.Equal(t, expect, out)

	// Add a destination via config entry and make sure it's picked up by the wildcard.
	configEntryDest := &structs.ServiceConfigEntry{
		Kind:        structs.ServiceDefaults,
		Name:        "destination1",
		Destination: &structs.DestinationConfig{Port: 9000, Addresses: []string{"kafka.test.com"}},
	}
	assert.NoError(t, s.EnsureConfigEntry(27, configEntryDest))

	idx, out, err = s.GatewayServices(ws, "gateway2", nil)
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(27))
	assert.Len(t, out, 3)

	expectWildcardIncludesDest := structs.GatewayServices{
		{
			Service:      structs.NewServiceName("api", nil),
			Gateway:      structs.NewServiceName("gateway2", nil),
			GatewayKind:  structs.ServiceKindTerminatingGateway,
			FromWildcard: true,
			RaftIndex: structs.RaftIndex{
				CreateIndex: 26,
				ModifyIndex: 26,
			},
		},
		{
			Service:      structs.NewServiceName("db", nil),
			Gateway:      structs.NewServiceName("gateway2", nil),
			GatewayKind:  structs.ServiceKindTerminatingGateway,
			FromWildcard: true,
			RaftIndex: structs.RaftIndex{
				CreateIndex: 26,
				ModifyIndex: 26,
			},
		},
		{
			Service:      structs.NewServiceName("destination1", nil),
			Gateway:      structs.NewServiceName("gateway2", nil),
			GatewayKind:  structs.ServiceKindTerminatingGateway,
			ServiceKind:  structs.GatewayServiceKindDestination,
			FromWildcard: true,
			RaftIndex: structs.RaftIndex{
				CreateIndex: 27,
				ModifyIndex: 27,
			},
		},
	}
	assert.ElementsMatch(t, expectWildcardIncludesDest, out)

	// Delete the destination.
	assert.NoError(t, s.DeleteConfigEntry(28, structs.ServiceDefaults, "destination1", nil))

	idx, out, err = s.GatewayServices(ws, "gateway2", nil)
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(28))
	assert.Len(t, out, 2)
	assert.Equal(t, expect, out)

	// Deleting the config entry should remove existing mappings
	assert.Nil(t, s.DeleteConfigEntry(29, "terminating-gateway", "gateway", nil))
	assert.True(t, watchFired(ws))

	idx, out, err = s.GatewayServices(ws, "gateway", nil)
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(29))
	assert.Len(t, out, 0)
}

func TestStateStore_ServiceGateways_Terminating(t *testing.T) {
	s := testStateStore(t)

	// Listing with no results returns an empty list.
	ws := memdb.NewWatchSet()
	idx, nodes, err := s.GatewayServices(ws, "db", nil)
	assert.Nil(t, err)
	assert.Equal(t, uint64(0), idx)
	assert.Len(t, nodes, 0)

	// Create some nodes
	assert.Nil(t, s.EnsureNode(10, &structs.Node{Node: "foo", Address: "127.0.0.1"}))
	assert.Nil(t, s.EnsureNode(11, &structs.Node{Node: "bar", Address: "127.0.0.2"}))
	assert.Nil(t, s.EnsureNode(12, &structs.Node{Node: "baz", Address: "127.0.0.2"}))

	// Typical services and some consul services spread across two nodes
	assert.Nil(t, s.EnsureService(13, "foo", &structs.NodeService{ID: "db", Service: "db", Tags: nil, Address: "", Port: 5000}))
	assert.Nil(t, s.EnsureService(15, "bar", &structs.NodeService{ID: "api", Service: "api", Tags: nil, Address: "", Port: 5000}))
	assert.Nil(t, s.EnsureService(16, "bar", &structs.NodeService{ID: "consul", Service: "consul", Tags: nil}))
	assert.Nil(t, s.EnsureService(17, "bar", &structs.NodeService{ID: "consul", Service: "consul", Tags: nil}))

	// Add ingress gateway and a connect proxy, neither should get picked up by terminating gateway
	ingressNS := &structs.NodeService{
		Kind:    structs.ServiceKindIngressGateway,
		ID:      "ingress",
		Service: "ingress",
		Port:    8443,
	}
	assert.Nil(t, s.EnsureService(18, "baz", ingressNS))

	proxyNS := &structs.NodeService{
		Kind:    structs.ServiceKindConnectProxy,
		ID:      "db proxy",
		Service: "db proxy",
		Proxy: structs.ConnectProxyConfig{
			DestinationServiceName: "db",
		},
		Port: 8000,
	}
	assert.Nil(t, s.EnsureService(19, "foo", proxyNS))

	// Register a gateway
	assert.Nil(t, s.EnsureService(20, "baz", &structs.NodeService{Kind: structs.ServiceKindTerminatingGateway, ID: "gateway", Service: "gateway", Port: 443}))

	// Associate gateway with db and api
	assert.Nil(t, s.EnsureConfigEntry(21, &structs.TerminatingGatewayConfigEntry{
		Kind: "terminating-gateway",
		Name: "gateway",
		Services: []structs.LinkedService{
			{
				Name: "db",
			},
			{
				Name: "api",
			},
		},
	}))
	assert.True(t, watchFired(ws))

	// Read everything back.
	ws = memdb.NewWatchSet()
	idx, out, err := s.ServiceGateways(ws, "db", structs.ServiceKindTerminatingGateway, *structs.DefaultEnterpriseMetaInDefaultPartition())
	assert.Nil(t, err)
	assert.Equal(t, uint64(21), idx)
	assert.Len(t, out, 1)

	expect := structs.CheckServiceNodes{
		{
			Node: &structs.Node{
				ID:        "",
				Address:   "127.0.0.2",
				Node:      "baz",
				Partition: acl.DefaultPartitionName,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 12,
					ModifyIndex: 12,
				},
			},
			Service: &structs.NodeService{
				Service:        "gateway",
				Kind:           structs.ServiceKindTerminatingGateway,
				ID:             "gateway",
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				Weights:        &structs.Weights{Passing: 1, Warning: 1},
				Port:           443,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 20,
					ModifyIndex: 20,
				},
			},
		},
	}
	assert.Equal(t, expect, out)

	// Check that we don't update on same exact config
	assert.Nil(t, s.EnsureConfigEntry(21, &structs.TerminatingGatewayConfigEntry{
		Kind: "terminating-gateway",
		Name: "gateway",
		Services: []structs.LinkedService{
			{
				Name: "db",
			},
			{
				Name: "api",
			},
		},
	}))
	assert.False(t, watchFired(ws))

	idx, out, err = s.ServiceGateways(ws, "api", structs.ServiceKindTerminatingGateway, *structs.DefaultEnterpriseMetaInDefaultPartition())
	assert.Nil(t, err)
	assert.Equal(t, uint64(21), idx)
	assert.Len(t, out, 1)

	expect = structs.CheckServiceNodes{
		{
			Node: &structs.Node{
				ID:        "",
				Address:   "127.0.0.2",
				Node:      "baz",
				Partition: acl.DefaultPartitionName,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 12,
					ModifyIndex: 12,
				},
			},
			Service: &structs.NodeService{
				Service:        "gateway",
				Kind:           structs.ServiceKindTerminatingGateway,
				ID:             "gateway",
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				Weights:        &structs.Weights{Passing: 1, Warning: 1},
				Port:           443,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 20,
					ModifyIndex: 20,
				},
			},
		},
	}
	assert.Equal(t, expect, out)

	// Associate gateway with a wildcard and add TLS config
	assert.Nil(t, s.EnsureConfigEntry(22, &structs.TerminatingGatewayConfigEntry{
		Kind: "terminating-gateway",
		Name: "gateway",
		Services: []structs.LinkedService{
			{
				Name:     "api",
				CAFile:   "api/ca.crt",
				CertFile: "api/client.crt",
				KeyFile:  "api/client.key",
				SNI:      "my-domain",
			},
			{
				Name: "db",
			},
			{
				Name:     "*",
				CAFile:   "ca.crt",
				CertFile: "client.crt",
				KeyFile:  "client.key",
				SNI:      "my-alt-domain",
			},
		},
	}))
	assert.True(t, watchFired(ws))

	// Read everything back.
	ws = memdb.NewWatchSet()
	idx, out, err = s.ServiceGateways(ws, "db", structs.ServiceKindTerminatingGateway, *structs.DefaultEnterpriseMetaInDefaultPartition())
	assert.Nil(t, err)
	assert.Equal(t, uint64(22), idx)
	assert.Len(t, out, 1)

	expect = structs.CheckServiceNodes{
		{
			Node: &structs.Node{
				ID:        "",
				Address:   "127.0.0.2",
				Node:      "baz",
				Partition: acl.DefaultPartitionName,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 12,
					ModifyIndex: 12,
				},
			},
			Service: &structs.NodeService{
				Service:        "gateway",
				Kind:           structs.ServiceKindTerminatingGateway,
				ID:             "gateway",
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				Weights:        &structs.Weights{Passing: 1, Warning: 1},
				Port:           443,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 20,
					ModifyIndex: 20,
				},
			},
		},
	}
	assert.Equal(t, expect, out)

	// Add a service covered by wildcard
	assert.Nil(t, s.EnsureService(23, "bar", &structs.NodeService{ID: "redis", Service: "redis", Tags: nil, Address: "", Port: 6379}))

	ws = memdb.NewWatchSet()
	idx, out, err = s.ServiceGateways(ws, "redis", structs.ServiceKindTerminatingGateway, *structs.DefaultEnterpriseMetaInDefaultPartition())
	assert.Nil(t, err)
	assert.Equal(t, uint64(23), idx)
	assert.Len(t, out, 1)

	expect = structs.CheckServiceNodes{
		{
			Node: &structs.Node{
				ID:        "",
				Address:   "127.0.0.2",
				Node:      "baz",
				Partition: acl.DefaultPartitionName,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 12,
					ModifyIndex: 12,
				},
			},
			Service: &structs.NodeService{
				Service:        "gateway",
				Kind:           structs.ServiceKindTerminatingGateway,
				ID:             "gateway",
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				Weights:        &structs.Weights{Passing: 1, Warning: 1},
				Port:           443,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 20,
					ModifyIndex: 20,
				},
			},
		},
	}
	assert.Equal(t, expect, out)

	// Delete a service covered by wildcard
	assert.Nil(t, s.DeleteService(24, "bar", "redis", structs.DefaultEnterpriseMetaInDefaultPartition(), ""))
	assert.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	idx, out, err = s.ServiceGateways(ws, "redis", structs.ServiceKindTerminatingGateway, *structs.DefaultEnterpriseMetaInDefaultPartition())
	assert.Nil(t, err)
	// TODO: wildcards don't keep the same extinction index
	assert.Equal(t, uint64(0), idx)
	assert.Len(t, out, 0)

	// Update the entry that only leaves one service
	assert.Nil(t, s.EnsureConfigEntry(25, &structs.TerminatingGatewayConfigEntry{
		Kind: "terminating-gateway",
		Name: "gateway",
		Services: []structs.LinkedService{
			{
				Name: "db",
			},
		},
	}))
	assert.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	idx, out, err = s.ServiceGateways(ws, "db", structs.ServiceKindTerminatingGateway, *structs.DefaultEnterpriseMetaInDefaultPartition())
	assert.Nil(t, err)
	assert.Equal(t, uint64(25), idx)
	assert.Len(t, out, 1)

	// previously associated services should not be present
	expect = structs.CheckServiceNodes{
		{
			Node: &structs.Node{
				ID:        "",
				Address:   "127.0.0.2",
				Node:      "baz",
				Partition: acl.DefaultPartitionName,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 12,
					ModifyIndex: 12,
				},
			},
			Service: &structs.NodeService{
				Service:        "gateway",
				Kind:           structs.ServiceKindTerminatingGateway,
				ID:             "gateway",
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				Weights:        &structs.Weights{Passing: 1, Warning: 1},
				Port:           443,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 20,
					ModifyIndex: 20,
				},
			},
		},
	}
	assert.Equal(t, expect, out)

	// Attempt to associate a different gateway with services that include db
	assert.Nil(t, s.EnsureConfigEntry(26, &structs.TerminatingGatewayConfigEntry{
		Kind: "terminating-gateway",
		Name: "gateway2",
		Services: []structs.LinkedService{
			{
				Name: "*",
			},
		},
	}))

	// check that watchset fired for new terminating gateway node service
	assert.Nil(t, s.EnsureService(20, "baz", &structs.NodeService{Kind: structs.ServiceKindTerminatingGateway, ID: "gateway2", Service: "gateway2", Port: 443}))
	assert.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	idx, out, err = s.ServiceGateways(ws, "db", structs.ServiceKindTerminatingGateway, *structs.DefaultEnterpriseMetaInDefaultPartition())
	assert.Nil(t, err)
	assert.Equal(t, uint64(26), idx)
	assert.Len(t, out, 2)

	expect = structs.CheckServiceNodes{
		{
			Node: &structs.Node{
				ID:        "",
				Address:   "127.0.0.2",
				Node:      "baz",
				Partition: acl.DefaultPartitionName,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 12,
					ModifyIndex: 12,
				},
			},
			Service: &structs.NodeService{
				Service:        "gateway",
				Kind:           structs.ServiceKindTerminatingGateway,
				ID:             "gateway",
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				Weights:        &structs.Weights{Passing: 1, Warning: 1},
				Port:           443,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 20,
					ModifyIndex: 20,
				},
			},
		},
		{
			Node: &structs.Node{
				ID:        "",
				Address:   "127.0.0.2",
				Node:      "baz",
				Partition: acl.DefaultPartitionName,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 12,
					ModifyIndex: 12,
				},
			},
			Service: &structs.NodeService{
				Service:        "gateway2",
				Kind:           structs.ServiceKindTerminatingGateway,
				ID:             "gateway2",
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				Weights:        &structs.Weights{Passing: 1, Warning: 1},
				Port:           443,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 20,
					ModifyIndex: 20,
				},
			},
		},
	}
	assert.Equal(t, expect, out)

	// Deleting the all gateway's node services should trigger the watch and keep the raft index stable
	assert.Nil(t, s.DeleteService(27, "baz", "gateway", structs.DefaultEnterpriseMetaInDefaultPartition(), structs.DefaultPeerKeyword))
	assert.True(t, watchFired(ws))
	assert.Nil(t, s.DeleteService(28, "baz", "gateway2", structs.DefaultEnterpriseMetaInDefaultPartition(), structs.DefaultPeerKeyword))

	ws = memdb.NewWatchSet()
	idx, out, err = s.ServiceGateways(ws, "db", structs.ServiceKindTerminatingGateway, *structs.DefaultEnterpriseMetaInDefaultPartition())
	assert.Nil(t, err)
	assert.Equal(t, uint64(28), idx)
	assert.Len(t, out, 0)

	// Deleting the config entry even with a node service should remove existing mappings
	assert.Nil(t, s.EnsureService(29, "baz", &structs.NodeService{Kind: structs.ServiceKindTerminatingGateway, ID: "gateway", Service: "gateway", Port: 443}))
	assert.Nil(t, s.DeleteConfigEntry(30, "terminating-gateway", "gateway", nil))
	assert.True(t, watchFired(ws))

	idx, out, err = s.ServiceGateways(ws, "api", structs.ServiceKindTerminatingGateway, *structs.DefaultEnterpriseMetaInDefaultPartition())
	assert.Nil(t, err)
	// TODO: similar to ingress, the index can backslide if the config is deleted.
	assert.Equal(t, uint64(28), idx)
	assert.Len(t, out, 0)
}

func TestStateStore_GatewayServices_ServiceDeletion(t *testing.T) {
	s := testStateStore(t)

	// Create some nodes
	assert.Nil(t, s.EnsureNode(10, &structs.Node{Node: "foo", Address: "127.0.0.1"}))
	assert.Nil(t, s.EnsureNode(11, &structs.Node{Node: "bar", Address: "127.0.0.2"}))
	assert.Nil(t, s.EnsureNode(12, &structs.Node{Node: "baz", Address: "127.0.0.2"}))

	// Typical services and some consul services spread across two nodes
	assert.Nil(t, s.EnsureService(13, "foo", &structs.NodeService{ID: "db", Service: "db", Tags: nil, Address: "", Port: 5000}))
	assert.Nil(t, s.EnsureService(14, "foo", &structs.NodeService{ID: "api", Service: "api", Tags: nil, Address: "", Port: 5000}))

	// Connect services (should be ignored by terminating gateway)
	assert.Nil(t, s.EnsureService(15, "foo", &structs.NodeService{ID: "web", Service: "web", Tags: nil, Address: "", Connect: structs.ServiceConnect{Native: true}, Port: 5000}))
	assert.Nil(t, s.EnsureService(16, "bar", &structs.NodeService{ID: "api", Service: "api", Tags: nil, Address: "", Connect: structs.ServiceConnect{Native: true}, Port: 5000}))

	// Register two gateways
	assert.Nil(t, s.EnsureService(17, "bar", &structs.NodeService{Kind: structs.ServiceKindTerminatingGateway, ID: "gateway", Service: "gateway", Port: 443}))
	assert.Nil(t, s.EnsureService(18, "baz", &structs.NodeService{Kind: structs.ServiceKindTerminatingGateway, ID: "other-gateway", Service: "other-gateway", Port: 443}))

	// Listing with no results returns an empty list.
	ws := memdb.NewWatchSet()
	idx, nodes, err := s.GatewayServices(ws, "gateway", nil)
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(0))
	assert.Len(t, nodes, 0)

	// Associate the first gateway with db
	assert.Nil(t, s.EnsureConfigEntry(19, &structs.TerminatingGatewayConfigEntry{
		Kind: "terminating-gateway",
		Name: "gateway",
		Services: []structs.LinkedService{
			{
				Name:   "db",
				CAFile: "my_ca.pem",
			},
		},
	}))
	assert.True(t, watchFired(ws))

	// Listing with no results returns an empty list.
	otherWS := memdb.NewWatchSet()
	idx, _, err = s.GatewayServices(otherWS, "other-gateway", nil)
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(19))
	assert.Len(t, nodes, 0)

	// Associate the second gateway with wildcard
	assert.Nil(t, s.EnsureConfigEntry(20, &structs.TerminatingGatewayConfigEntry{
		Kind: "terminating-gateway",
		Name: "other-gateway",
		Services: []structs.LinkedService{
			{
				Name: "*",
			},
		},
	}))
	assert.True(t, watchFired(ws))

	// Read everything back for first gateway.
	ws = memdb.NewWatchSet()
	idx, out, err := s.GatewayServices(ws, "gateway", nil)
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(20))
	assert.Len(t, out, 1)

	expect := structs.GatewayServices{
		{
			Service:     structs.NewServiceName("db", nil),
			Gateway:     structs.NewServiceName("gateway", nil),
			GatewayKind: structs.ServiceKindTerminatingGateway,
			CAFile:      "my_ca.pem",
			RaftIndex: structs.RaftIndex{
				CreateIndex: 19,
				ModifyIndex: 19,
			},
			ServiceKind: structs.GatewayServiceKindService,
		},
	}
	assert.Equal(t, expect, out)

	// Read everything back for other gateway.
	otherWS = memdb.NewWatchSet()
	idx, out, err = s.GatewayServices(otherWS, "other-gateway", nil)
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(20))
	assert.Len(t, out, 2)

	expect = structs.GatewayServices{
		{
			Service:      structs.NewServiceName("api", nil),
			Gateway:      structs.NewServiceName("other-gateway", nil),
			GatewayKind:  structs.ServiceKindTerminatingGateway,
			FromWildcard: true,
			RaftIndex: structs.RaftIndex{
				CreateIndex: 20,
				ModifyIndex: 20,
			},
		},
		{
			Service:      structs.NewServiceName("db", nil),
			Gateway:      structs.NewServiceName("other-gateway", nil),
			GatewayKind:  structs.ServiceKindTerminatingGateway,
			FromWildcard: true,
			RaftIndex: structs.RaftIndex{
				CreateIndex: 20,
				ModifyIndex: 20,
			},
		},
	}
	assert.Equal(t, expect, out)

	// Delete a service specified directly.
	assert.Nil(t, s.DeleteService(20, "foo", "db", nil, ""))

	// The watch will fire because we need to update the gateway-services kind
	assert.True(t, watchFired(ws))
	assert.True(t, watchFired(otherWS))

	// db should remain in the original gateway
	idx, out, err = s.GatewayServices(ws, "gateway", nil)
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(20))
	assert.Len(t, out, 1)

	expect = structs.GatewayServices{
		{
			Service:     structs.NewServiceName("db", nil),
			Gateway:     structs.NewServiceName("gateway", nil),
			GatewayKind: structs.ServiceKindTerminatingGateway,
			CAFile:      "my_ca.pem",
			RaftIndex: structs.RaftIndex{
				CreateIndex: 19,
				ModifyIndex: 20,
			},
		},
	}
	assert.Equal(t, expect, out)

	// db should not have been deleted from the other gateway
	idx, out, err = s.GatewayServices(ws, "other-gateway", nil)
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(20))
	assert.Len(t, out, 1)

	expect = structs.GatewayServices{
		{
			Service:      structs.NewServiceName("api", nil),
			Gateway:      structs.NewServiceName("other-gateway", nil),
			GatewayKind:  structs.ServiceKindTerminatingGateway,
			FromWildcard: true,
			RaftIndex: structs.RaftIndex{
				CreateIndex: 20,
				ModifyIndex: 20,
			},
		},
	}
	assert.Equal(t, expect, out)

	// Delete the non-connect instance of api
	assert.Nil(t, s.DeleteService(21, "foo", "api", nil, ""))

	// Gateway with wildcard entry should have no services left, because the last
	// non-connect instance of 'api' was deleted.
	idx, out, err = s.GatewayServices(ws, "other-gateway", nil)
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(21))
	assert.Empty(t, out)
}

func TestStateStore_CheckIngressServiceNodes(t *testing.T) {
	s := testStateStore(t)
	ws := setupIngressState(t, s)

	t.Run("check service1 ingress gateway", func(t *testing.T) {
		idx, results, err := s.CheckIngressServiceNodes(ws, "service1", nil)
		require.NoError(t, err)
		require.Equal(t, uint64(18), idx)
		// Multiple instances of the ingress2 service
		require.Len(t, results, 4)

		ids := make(map[string]struct{})
		for _, n := range results {
			ids[n.Service.ID] = struct{}{}
		}
		expectedIds := map[string]struct{}{
			"ingress1":        {},
			"ingress2":        {},
			"wildcardIngress": {},
		}
		require.Equal(t, expectedIds, ids)
	})

	t.Run("check service2 ingress gateway", func(t *testing.T) {
		idx, results, err := s.CheckIngressServiceNodes(ws, "service2", nil)
		require.NoError(t, err)
		require.Equal(t, uint64(18), idx)
		require.Len(t, results, 2)

		ids := make(map[string]struct{})
		for _, n := range results {
			ids[n.Service.ID] = struct{}{}
		}
		expectedIds := map[string]struct{}{
			"ingress1":        {},
			"wildcardIngress": {},
		}
		require.Equal(t, expectedIds, ids)
	})

	t.Run("check service3 ingress gateway", func(t *testing.T) {
		ws := memdb.NewWatchSet()
		idx, results, err := s.CheckIngressServiceNodes(ws, "service3", nil)
		require.NoError(t, err)
		require.Equal(t, uint64(18), idx)
		require.Len(t, results, 1)
		require.Equal(t, "wildcardIngress", results[0].Service.ID)
	})

	t.Run("delete a wildcard entry", func(t *testing.T) {
		require.Nil(t, s.DeleteConfigEntry(19, "ingress-gateway", "wildcardIngress", nil))
		require.True(t, watchFired(ws))

		idx, results, err := s.CheckIngressServiceNodes(ws, "service1", nil)
		require.NoError(t, err)
		require.Equal(t, uint64(18), idx)
		require.Len(t, results, 3)

		idx, results, err = s.CheckIngressServiceNodes(ws, "service2", nil)
		require.NoError(t, err)
		require.Equal(t, uint64(18), idx)
		require.Len(t, results, 1)

		idx, results, err = s.CheckIngressServiceNodes(ws, "service3", nil)
		require.NoError(t, err)
		require.Equal(t, uint64(18), idx)
		// TODO(ingress): index goes backward when deleting last config entry
		// require.Equal(t,uint64(11), idx)
		require.Len(t, results, 0)
	})
}

func TestStateStore_GatewayServices_Ingress(t *testing.T) {
	s := testStateStore(t)
	ws := setupIngressState(t, s)

	t.Run("ingress1 gateway services", func(t *testing.T) {
		expected := structs.GatewayServices{
			{
				Gateway:     structs.NewServiceName("ingress1", nil),
				Service:     structs.NewServiceName("service1", nil),
				GatewayKind: structs.ServiceKindIngressGateway,
				Port:        1111,
				Protocol:    "http",
				Hosts:       []string{"test.example.com"},
				RaftIndex: structs.RaftIndex{
					CreateIndex: 13,
					ModifyIndex: 13,
				},
			},
			{
				Gateway:     structs.NewServiceName("ingress1", nil),
				Service:     structs.NewServiceName("service2", nil),
				GatewayKind: structs.ServiceKindIngressGateway,
				Port:        2222,
				Protocol:    "http",
				RaftIndex: structs.RaftIndex{
					CreateIndex: 13,
					ModifyIndex: 13,
				},
			},
		}
		idx, results, err := s.GatewayServices(ws, "ingress1", nil)
		require.NoError(t, err)
		require.Equal(t, uint64(16), idx)
		require.ElementsMatch(t, results, expected)
	})

	t.Run("ingress2 gateway services", func(t *testing.T) {
		expected := structs.GatewayServices{
			{
				Gateway:     structs.NewServiceName("ingress2", nil),
				Service:     structs.NewServiceName("service1", nil),
				GatewayKind: structs.ServiceKindIngressGateway,
				Port:        3333,
				Protocol:    "http",
				RaftIndex: structs.RaftIndex{
					CreateIndex: 14,
					ModifyIndex: 14,
				},
			},
		}
		idx, results, err := s.GatewayServices(ws, "ingress2", nil)
		require.NoError(t, err)
		require.Equal(t, uint64(16), idx)
		require.ElementsMatch(t, results, expected)
	})

	t.Run("No gatway services associated", func(t *testing.T) {
		idx, results, err := s.GatewayServices(ws, "nothingIngress", nil)
		require.NoError(t, err)
		require.Equal(t, uint64(16), idx)
		require.Len(t, results, 0)
	})

	t.Run("wildcard gateway services", func(t *testing.T) {
		expected := structs.GatewayServices{
			{
				Gateway:      structs.NewServiceName("wildcardIngress", nil),
				Service:      structs.NewServiceName("service1", nil),
				GatewayKind:  structs.ServiceKindIngressGateway,
				Port:         4444,
				Protocol:     "http",
				FromWildcard: true,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 12,
					ModifyIndex: 12,
				},
			},
			{
				Gateway:      structs.NewServiceName("wildcardIngress", nil),
				Service:      structs.NewServiceName("service2", nil),
				GatewayKind:  structs.ServiceKindIngressGateway,
				Port:         4444,
				Protocol:     "http",
				FromWildcard: true,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 12,
					ModifyIndex: 12,
				},
			},
			{
				Gateway:      structs.NewServiceName("wildcardIngress", nil),
				Service:      structs.NewServiceName("service3", nil),
				GatewayKind:  structs.ServiceKindIngressGateway,
				Port:         4444,
				Protocol:     "http",
				FromWildcard: true,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 12,
					ModifyIndex: 12,
				},
			},
		}
		idx, results, err := s.GatewayServices(ws, "wildcardIngress", nil)
		require.NoError(t, err)
		require.Equal(t, uint64(16), idx)
		require.ElementsMatch(t, results, expected)
	})

	t.Run("gateway with duplicate service", func(t *testing.T) {
		expected := structs.GatewayServices{
			{
				Gateway:      structs.NewServiceName("ingress3", nil),
				Service:      structs.NewServiceName("service1", nil),
				GatewayKind:  structs.ServiceKindIngressGateway,
				Port:         5555,
				Protocol:     "http",
				FromWildcard: true,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 15,
					ModifyIndex: 15,
				},
			},
			{
				Gateway:      structs.NewServiceName("ingress3", nil),
				Service:      structs.NewServiceName("service2", nil),
				GatewayKind:  structs.ServiceKindIngressGateway,
				Port:         5555,
				Protocol:     "http",
				FromWildcard: true,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 15,
					ModifyIndex: 15,
				},
			},
			{
				Gateway:      structs.NewServiceName("ingress3", nil),
				Service:      structs.NewServiceName("service3", nil),
				GatewayKind:  structs.ServiceKindIngressGateway,
				Port:         5555,
				Protocol:     "http",
				FromWildcard: true,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 15,
					ModifyIndex: 15,
				},
			},
			{
				Gateway:     structs.NewServiceName("ingress3", nil),
				Service:     structs.NewServiceName("service1", nil),
				GatewayKind: structs.ServiceKindIngressGateway,
				Port:        6666,
				Protocol:    "http",
				RaftIndex: structs.RaftIndex{
					CreateIndex: 15,
					ModifyIndex: 15,
				},
			},
		}
		idx, results, err := s.GatewayServices(ws, "ingress3", nil)
		require.NoError(t, err)
		require.Equal(t, uint64(16), idx)
		require.ElementsMatch(t, results, expected)
	})

	t.Run("deregistering a service", func(t *testing.T) {
		require.Nil(t, s.DeleteService(18, "node1", "service1", nil, ""))
		require.True(t, watchFired(ws))

		ws = memdb.NewWatchSet()
		idx, results, err := s.GatewayServices(ws, "wildcardIngress", nil)
		require.NoError(t, err)
		require.Equal(t, uint64(18), idx)
		require.Len(t, results, 2)
	})

	t.Run("check ingress2 gateway services again", func(t *testing.T) {
		expected := structs.GatewayServices{
			{
				Gateway:     structs.NewServiceName("ingress2", nil),
				Service:     structs.NewServiceName("service1", nil),
				GatewayKind: structs.ServiceKindIngressGateway,
				Port:        3333,
				Protocol:    "http",
				RaftIndex: structs.RaftIndex{
					CreateIndex: 14,
					ModifyIndex: 14,
				},
			},
		}
		ws = memdb.NewWatchSet()
		idx, results, err := s.GatewayServices(ws, "ingress2", nil)
		require.NoError(t, err)
		require.Equal(t, uint64(18), idx)
		require.ElementsMatch(t, results, expected)
	})

	t.Run("deleting a wildcard config entry", func(t *testing.T) {
		ws = memdb.NewWatchSet()
		_, _, err := s.GatewayServices(ws, "wildcardIngress", nil)
		require.NoError(t, err)

		require.Nil(t, s.DeleteConfigEntry(19, "ingress-gateway", "wildcardIngress", nil))
		require.True(t, watchFired(ws))

		ws = memdb.NewWatchSet()
		idx, results, err := s.GatewayServices(ws, "wildcardIngress", nil)
		require.NoError(t, err)
		require.Equal(t, uint64(19), idx)
		require.Len(t, results, 0)
	})

	t.Run("update ingress1 with exact same config entry", func(t *testing.T) {
		ingress1 := &structs.IngressGatewayConfigEntry{
			Kind: "ingress-gateway",
			Name: "ingress1",
			Listeners: []structs.IngressListener{
				{
					Port:     1111,
					Protocol: "http",
					Services: []structs.IngressService{
						{
							Name:  "service1",
							Hosts: []string{"test.example.com"},
						},
					},
				},
				{
					Port:     2222,
					Protocol: "http",
					Services: []structs.IngressService{
						{
							Name: "service2",
						},
					},
				},
			},
		}

		ws = memdb.NewWatchSet()
		_, _, err := s.GatewayServices(ws, "ingress1", nil)
		require.NoError(t, err)

		require.Nil(t, s.EnsureConfigEntry(20, ingress1))
		require.False(t, watchFired(ws))

		expected := structs.GatewayServices{
			{
				Gateway:     structs.NewServiceName("ingress1", nil),
				Service:     structs.NewServiceName("service1", nil),
				GatewayKind: structs.ServiceKindIngressGateway,
				Port:        1111,
				Protocol:    "http",
				Hosts:       []string{"test.example.com"},
				RaftIndex: structs.RaftIndex{
					CreateIndex: 13,
					ModifyIndex: 13,
				},
			},
			{
				Gateway:     structs.NewServiceName("ingress1", nil),
				Service:     structs.NewServiceName("service2", nil),
				GatewayKind: structs.ServiceKindIngressGateway,
				Port:        2222,
				Protocol:    "http",
				RaftIndex: structs.RaftIndex{
					CreateIndex: 13,
					ModifyIndex: 13,
				},
			},
		}
		idx, results, err := s.GatewayServices(ws, "ingress1", nil)
		require.NoError(t, err)
		require.Equal(t, uint64(19), idx)
		require.ElementsMatch(t, results, expected)
	})

	t.Run("updating a config entry with zero listeners", func(t *testing.T) {
		ingress1 := &structs.IngressGatewayConfigEntry{
			Kind:      "ingress-gateway",
			Name:      "ingress1",
			Listeners: []structs.IngressListener{},
		}

		ws = memdb.NewWatchSet()
		_, _, err := s.GatewayServices(ws, "ingress1", nil)
		require.NoError(t, err)

		require.Nil(t, s.EnsureConfigEntry(20, ingress1))
		require.True(t, watchFired(ws))

		idx, results, err := s.GatewayServices(ws, "ingress1", nil)
		require.NoError(t, err)
		require.Equal(t, uint64(20), idx)
		require.Len(t, results, 0)
	})
}

func TestStateStore_GatewayServices_WildcardAssociation(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	s := testStateStore(t)
	setupIngressState(t, s)
	ws := memdb.NewWatchSet()

	t.Run("base case for wildcard", func(t *testing.T) {
		idx, results, err := s.GatewayServices(ws, "wildcardIngress", nil)
		require.NoError(t, err)
		require.Equal(t, uint64(16), idx)
		require.Len(t, results, 3)
	})

	t.Run("do not associate ingress services with gateway", func(t *testing.T) {
		testRegisterIngressService(t, s, 17, "node1", "testIngress")
		require.False(t, watchFired(ws))
		idx, results, err := s.GatewayServices(ws, "wildcardIngress", nil)
		require.NoError(t, err)
		require.Equal(t, uint64(16), idx)
		require.Len(t, results, 3)
	})

	t.Run("do not associate terminating-gateway services with gateway", func(t *testing.T) {
		require.Nil(t, s.EnsureService(18, "node1",
			&structs.NodeService{
				Kind: structs.ServiceKindTerminatingGateway, ID: "gateway", Service: "gateway", Port: 443,
			},
		))
		require.False(t, watchFired(ws))
		idx, results, err := s.GatewayServices(ws, "wildcardIngress", nil)
		require.NoError(t, err)
		require.Equal(t, uint64(16), idx)
		require.Len(t, results, 3)
	})

	t.Run("do not associate connect-proxy services with gateway", func(t *testing.T) {
		// Should only associate web (the destination service of the proxy), not the
		// sidecar service name itself.
		testRegisterSidecarProxy(t, s, 19, "node1", "web")
		expected := structs.GatewayServices{
			{
				Gateway:      structs.NewServiceName("wildcardIngress", nil),
				Service:      structs.NewServiceName("service1", nil),
				GatewayKind:  structs.ServiceKindIngressGateway,
				Port:         4444,
				Protocol:     "http",
				FromWildcard: true,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 12,
					ModifyIndex: 12,
				},
			},
			{
				Gateway:      structs.NewServiceName("wildcardIngress", nil),
				Service:      structs.NewServiceName("service2", nil),
				GatewayKind:  structs.ServiceKindIngressGateway,
				Port:         4444,
				Protocol:     "http",
				FromWildcard: true,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 12,
					ModifyIndex: 12,
				},
			},
			{
				Gateway:      structs.NewServiceName("wildcardIngress", nil),
				Service:      structs.NewServiceName("service3", nil),
				GatewayKind:  structs.ServiceKindIngressGateway,
				Port:         4444,
				Protocol:     "http",
				FromWildcard: true,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 12,
					ModifyIndex: 12,
				},
			},
			{
				Gateway:      structs.NewServiceName("wildcardIngress", nil),
				Service:      structs.NewServiceName("web", nil),
				ServiceKind:  structs.GatewayServiceKindService,
				GatewayKind:  structs.ServiceKindIngressGateway,
				Port:         4444,
				Protocol:     "http",
				FromWildcard: true,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 19,
					ModifyIndex: 19,
				},
			},
		}

		idx, results, err := s.GatewayServices(ws, "wildcardIngress", nil)
		require.NoError(t, err)
		require.Equal(t, uint64(19), idx)
		require.ElementsMatch(t, results, expected)
	})

	t.Run("do not associate consul services with gateway", func(t *testing.T) {
		ws := memdb.NewWatchSet()
		_, _, err := s.GatewayServices(ws, "wildcardIngress", nil)
		require.NoError(t, err)

		require.Nil(t, s.EnsureService(20, "node1",
			&structs.NodeService{ID: "consul", Service: "consul", Tags: nil},
		))
		require.False(t, watchFired(ws))
		idx, results, err := s.GatewayServices(ws, "wildcardIngress", nil)
		require.NoError(t, err)
		require.Equal(t, uint64(19), idx)
		require.Len(t, results, 4)
	})
}

func TestStateStore_GatewayServices_IngressProtocolFiltering(t *testing.T) {
	s := testStateStore(t)

	t.Run("setup", func(t *testing.T) {
		ingress1 := &structs.IngressGatewayConfigEntry{
			Kind: "ingress-gateway",
			Name: "ingress1",
			Listeners: []structs.IngressListener{
				{
					Port:     4444,
					Protocol: "http",
					Services: []structs.IngressService{
						{
							Name: "*",
						},
					},
				},
			},
		}

		testRegisterNode(t, s, 0, "node1")
		testRegisterConnectService(t, s, 1, "node1", "service1")
		testRegisterConnectService(t, s, 2, "node1", "service2")
		assert.NoError(t, s.EnsureConfigEntry(4, ingress1))
	})

	t.Run("no services from default tcp protocol", func(t *testing.T) {
		idx, results, err := s.GatewayServices(nil, "ingress1", nil)
		require.NoError(t, err)
		require.Equal(t, uint64(4), idx)
		require.Len(t, results, 0)
	})

	t.Run("service-defaults", func(t *testing.T) {
		expected := structs.GatewayServices{
			{
				Gateway:      structs.NewServiceName("ingress1", nil),
				Service:      structs.NewServiceName("service1", nil),
				GatewayKind:  structs.ServiceKindIngressGateway,
				Port:         4444,
				Protocol:     "http",
				FromWildcard: true,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 4,
					ModifyIndex: 4,
				},
			},
		}

		svcDefaults := &structs.ServiceConfigEntry{
			Name:     "service1",
			Kind:     structs.ServiceDefaults,
			Protocol: "http",
		}
		assert.NoError(t, s.EnsureConfigEntry(5, svcDefaults))
		idx, results, err := s.GatewayServices(nil, "ingress1", nil)
		require.NoError(t, err)
		require.Equal(t, uint64(5), idx)
		require.ElementsMatch(t, results, expected)
	})

	t.Run("proxy-defaults", func(t *testing.T) {
		expected := structs.GatewayServices{
			{
				Gateway:      structs.NewServiceName("ingress1", nil),
				Service:      structs.NewServiceName("service1", nil),
				GatewayKind:  structs.ServiceKindIngressGateway,
				Port:         4444,
				Protocol:     "http",
				FromWildcard: true,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 4,
					ModifyIndex: 4,
				},
			},
			{
				Gateway:      structs.NewServiceName("ingress1", nil),
				Service:      structs.NewServiceName("service2", nil),
				GatewayKind:  structs.ServiceKindIngressGateway,
				Port:         4444,
				Protocol:     "http",
				FromWildcard: true,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 4,
					ModifyIndex: 4,
				},
			},
		}

		proxyDefaults := &structs.ProxyConfigEntry{
			Name: structs.ProxyConfigGlobal,
			Kind: structs.ProxyDefaults,
			Config: map[string]interface{}{
				"protocol": "http",
			},
		}
		assert.NoError(t, s.EnsureConfigEntry(6, proxyDefaults))

		idx, results, err := s.GatewayServices(nil, "ingress1", nil)
		require.NoError(t, err)
		require.Equal(t, uint64(6), idx)
		require.ElementsMatch(t, results, expected)
	})

	t.Run("service-defaults overrides proxy-defaults", func(t *testing.T) {
		expected := structs.GatewayServices{
			{
				Gateway:      structs.NewServiceName("ingress1", nil),
				Service:      structs.NewServiceName("service2", nil),
				GatewayKind:  structs.ServiceKindIngressGateway,
				Port:         4444,
				Protocol:     "http",
				FromWildcard: true,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 4,
					ModifyIndex: 4,
				},
			},
		}

		svcDefaults := &structs.ServiceConfigEntry{
			Name:     "service1",
			Kind:     structs.ServiceDefaults,
			Protocol: "grpc",
		}
		assert.NoError(t, s.EnsureConfigEntry(7, svcDefaults))

		idx, results, err := s.GatewayServices(nil, "ingress1", nil)
		require.NoError(t, err)
		require.Equal(t, uint64(7), idx)
		require.ElementsMatch(t, results, expected)
	})

	t.Run("change listener protocol and expect different filter", func(t *testing.T) {
		expected := structs.GatewayServices{
			{
				Gateway:      structs.NewServiceName("ingress1", nil),
				Service:      structs.NewServiceName("service1", nil),
				GatewayKind:  structs.ServiceKindIngressGateway,
				Port:         4444,
				Protocol:     "grpc",
				FromWildcard: true,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 8,
					ModifyIndex: 8,
				},
			},
		}

		ingress1 := &structs.IngressGatewayConfigEntry{
			Kind: "ingress-gateway",
			Name: "ingress1",
			Listeners: []structs.IngressListener{
				{
					Port:     4444,
					Protocol: "grpc",
					Services: []structs.IngressService{
						{
							Name: "*",
						},
					},
				},
			},
		}
		assert.NoError(t, s.EnsureConfigEntry(8, ingress1))

		idx, results, err := s.GatewayServices(nil, "ingress1", nil)
		require.NoError(t, err)
		require.Equal(t, uint64(8), idx)
		require.ElementsMatch(t, results, expected)
	})
}

func setupIngressState(t *testing.T, s *Store) memdb.WatchSet {
	// Querying with no matches gives an empty response
	ws := memdb.NewWatchSet()
	idx, res, err := s.GatewayServices(ws, "ingress1", nil)
	if idx != 0 || res != nil || err != nil {
		t.Fatalf("expected (0, nil, nil), got: (%d, %#v, %#v)", idx, res, err)
	}

	// Register some nodes.
	testRegisterNode(t, s, 0, "node1")
	testRegisterNode(t, s, 1, "node2")

	// Register some connect services against the nodes.
	testRegisterIngressService(t, s, 3, "node1", "wildcardIngress")
	testRegisterIngressService(t, s, 4, "node1", "ingress1")
	testRegisterIngressService(t, s, 5, "node1", "ingress2")
	testRegisterIngressService(t, s, 6, "node2", "ingress2")
	testRegisterIngressService(t, s, 7, "node1", "nothingIngress")
	testRegisterConnectService(t, s, 8, "node1", "service1")
	testRegisterConnectService(t, s, 9, "node2", "service2")
	testRegisterService(t, s, 10, "node2", "service3")
	testRegisterServiceWithChangeOpts(t, s, 11, "node2", "service3-proxy", false, func(service *structs.NodeService) {
		service.Kind = structs.ServiceKindConnectProxy
		service.Proxy = structs.ConnectProxyConfig{
			DestinationServiceName: "service3",
		}
	})

	// Register some non-connect services - these shouldn't be picked up by a wildcard.
	testRegisterService(t, s, 17, "node1", "service4")
	testRegisterService(t, s, 18, "node2", "service5")

	// Default protocol to http
	proxyDefaults := &structs.ProxyConfigEntry{
		Name: structs.ProxyConfigGlobal,
		Kind: structs.ProxyDefaults,
		Config: map[string]interface{}{
			"protocol": "http",
		},
	}
	assert.NoError(t, s.EnsureConfigEntry(11, proxyDefaults))

	// Register some ingress config entries.
	wildcardIngress := &structs.IngressGatewayConfigEntry{
		Kind: "ingress-gateway",
		Name: "wildcardIngress",
		Listeners: []structs.IngressListener{
			{
				Port:     4444,
				Protocol: "http",
				Services: []structs.IngressService{
					{
						Name: "*",
					},
				},
			},
		},
	}
	assert.NoError(t, s.EnsureConfigEntry(12, wildcardIngress))

	ingress1 := &structs.IngressGatewayConfigEntry{
		Kind: "ingress-gateway",
		Name: "ingress1",
		Listeners: []structs.IngressListener{
			{
				Port:     1111,
				Protocol: "http",
				Services: []structs.IngressService{
					{
						Name:  "service1",
						Hosts: []string{"test.example.com"},
					},
				},
			},
			{
				Port:     2222,
				Protocol: "http",
				Services: []structs.IngressService{
					{
						Name: "service2",
					},
				},
			},
		},
	}
	assert.NoError(t, s.EnsureConfigEntry(13, ingress1))
	assert.True(t, watchFired(ws))

	ingress2 := &structs.IngressGatewayConfigEntry{
		Kind: "ingress-gateway",
		Name: "ingress2",
		Listeners: []structs.IngressListener{
			{
				Port:     3333,
				Protocol: "http",
				Services: []structs.IngressService{
					{
						Name: "service1",
					},
				},
			},
		},
	}
	assert.NoError(t, s.EnsureConfigEntry(14, ingress2))
	assert.True(t, watchFired(ws))

	ingress3 := &structs.IngressGatewayConfigEntry{
		Kind: "ingress-gateway",
		Name: "ingress3",
		Listeners: []structs.IngressListener{
			{
				Port:     5555,
				Protocol: "http",
				Services: []structs.IngressService{
					{
						Name: "*",
					},
				},
			},
			{
				Port:     6666,
				Protocol: "http",
				Services: []structs.IngressService{
					{
						Name: "service1",
					},
				},
			},
		},
	}
	assert.NoError(t, s.EnsureConfigEntry(15, ingress3))
	assert.True(t, watchFired(ws))

	nothingIngress := &structs.IngressGatewayConfigEntry{
		Kind:      "ingress-gateway",
		Name:      "nothingIngress",
		Listeners: []structs.IngressListener{},
	}
	assert.NoError(t, s.EnsureConfigEntry(16, nothingIngress))
	assert.True(t, watchFired(ws))

	return ws
}

func TestStore_EnsureService_DoesNotPanicOnIngressGateway(t *testing.T) {
	store := NewStateStore(nil)

	err := store.EnsureConfigEntry(1, &structs.IngressGatewayConfigEntry{
		Kind: structs.IngressGateway,
		Name: "the-ingress",
		Listeners: []structs.IngressListener{
			{
				Port:     12345,
				Protocol: "tcp",
				Services: []structs.IngressService{{Name: "the-service"}},
			},
		},
	})
	require.NoError(t, err)

	err = store.EnsureRegistration(2, &structs.RegisterRequest{
		Node: "the-node",
		Service: &structs.NodeService{
			Kind:    structs.ServiceKindConnectProxy,
			Service: "the-proxy",
			Proxy: structs.ConnectProxyConfig{
				DestinationServiceName: "the-ingress",
				Upstreams: []structs.Upstream{
					{
						DestinationName: "the-service",
					},
				},
			},
		},
	})
	require.NoError(t, err)
}

func TestStateStore_DumpGatewayServices(t *testing.T) {
	s := testStateStore(t)

	// Listing with no results returns an empty list.
	ws := memdb.NewWatchSet()
	idx, nodes, err := s.DumpGatewayServices(ws)
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(0))
	assert.Len(t, nodes, 0)

	// Create some nodes
	assert.Nil(t, s.EnsureNode(10, &structs.Node{Node: "foo", Address: "127.0.0.1"}))
	assert.Nil(t, s.EnsureNode(11, &structs.Node{Node: "bar", Address: "127.0.0.2"}))
	assert.Nil(t, s.EnsureNode(12, &structs.Node{Node: "baz", Address: "127.0.0.2"}))

	// Typical services and some consul services spread across two nodes
	assert.Nil(t, s.EnsureService(13, "foo", &structs.NodeService{ID: "db", Service: "db", Tags: nil, Address: "", Port: 5000}))
	assert.Nil(t, s.EnsureService(15, "bar", &structs.NodeService{ID: "api", Service: "api", Tags: nil, Address: "", Port: 5000}))
	assert.Nil(t, s.EnsureService(16, "bar", &structs.NodeService{ID: "consul", Service: "consul", Tags: nil}))
	assert.Nil(t, s.EnsureService(17, "bar", &structs.NodeService{ID: "consul", Service: "consul", Tags: nil}))

	ingressNS := &structs.NodeService{
		Kind:    structs.ServiceKindIngressGateway,
		ID:      "ingress",
		Service: "ingress",
		Port:    8443,
	}
	assert.Nil(t, s.EnsureService(18, "baz", ingressNS))

	// Register a gateway
	terminatingNS := &structs.NodeService{
		Kind:    structs.ServiceKindTerminatingGateway,
		ID:      "gateway",
		Service: "gateway",
		Port:    443,
	}
	assert.Nil(t, s.EnsureService(20, "baz", terminatingNS))

	t.Run("add-tgw-config", func(t *testing.T) {
		// Associate gateway with db and api
		assert.Nil(t, s.EnsureConfigEntry(21, &structs.TerminatingGatewayConfigEntry{
			Kind: "terminating-gateway",
			Name: "gateway",
			Services: []structs.LinkedService{
				{
					Name:     "api",
					CAFile:   "api/ca.crt",
					CertFile: "api/client.crt",
					KeyFile:  "api/client.key",
					SNI:      "my-domain",
				},
				{
					Name: "db",
				},
				{
					Name:     "*",
					CAFile:   "ca.crt",
					CertFile: "client.crt",
					KeyFile:  "client.key",
					SNI:      "my-alt-domain",
				},
			},
		}))
		assert.True(t, watchFired(ws))

		// Read everything back.
		ws = memdb.NewWatchSet()
		idx, out, err := s.DumpGatewayServices(ws)
		assert.Nil(t, err)
		assert.Equal(t, idx, uint64(21))
		assert.Len(t, out, 2)

		expect := structs.GatewayServices{
			{
				Service:     structs.NewServiceName("api", nil),
				Gateway:     structs.NewServiceName("gateway", nil),
				GatewayKind: structs.ServiceKindTerminatingGateway,
				CAFile:      "api/ca.crt",
				CertFile:    "api/client.crt",
				KeyFile:     "api/client.key",
				SNI:         "my-domain",
				RaftIndex: structs.RaftIndex{
					CreateIndex: 21,
					ModifyIndex: 21,
				},
				ServiceKind: structs.GatewayServiceKindService,
			},
			{
				Service:     structs.NewServiceName("db", nil),
				Gateway:     structs.NewServiceName("gateway", nil),
				GatewayKind: structs.ServiceKindTerminatingGateway,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 21,
					ModifyIndex: 21,
				},
				ServiceKind: structs.GatewayServiceKindService,
			},
		}
		assert.Equal(t, expect, out)
	})

	t.Run("no-op", func(t *testing.T) {
		// Check watch doesn't fire on same exact config
		assert.Nil(t, s.EnsureConfigEntry(21, &structs.TerminatingGatewayConfigEntry{
			Kind: "terminating-gateway",
			Name: "gateway",
			Services: []structs.LinkedService{
				{
					Name:     "api",
					CAFile:   "api/ca.crt",
					CertFile: "api/client.crt",
					KeyFile:  "api/client.key",
					SNI:      "my-domain",
				},
				{
					Name: "db",
				},
				{
					Name:     "*",
					CAFile:   "ca.crt",
					CertFile: "client.crt",
					KeyFile:  "client.key",
					SNI:      "my-alt-domain",
				},
			},
		}))
		assert.False(t, watchFired(ws))

		idx, out, err := s.DumpGatewayServices(ws)
		assert.Nil(t, err)
		assert.Equal(t, idx, uint64(21))
		assert.Len(t, out, 2)

		expect := structs.GatewayServices{
			{
				Service:     structs.NewServiceName("api", nil),
				Gateway:     structs.NewServiceName("gateway", nil),
				GatewayKind: structs.ServiceKindTerminatingGateway,
				CAFile:      "api/ca.crt",
				CertFile:    "api/client.crt",
				KeyFile:     "api/client.key",
				SNI:         "my-domain",
				RaftIndex: structs.RaftIndex{
					CreateIndex: 21,
					ModifyIndex: 21,
				},
				ServiceKind: structs.GatewayServiceKindService,
			},
			{
				Service:     structs.NewServiceName("db", nil),
				Gateway:     structs.NewServiceName("gateway", nil),
				GatewayKind: structs.ServiceKindTerminatingGateway,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 21,
					ModifyIndex: 21,
				},
				ServiceKind: structs.GatewayServiceKindService,
			},
		}
		assert.Equal(t, expect, out)
	})

	// Add a service covered by wildcard
	t.Run("add-wc-service", func(t *testing.T) {
		assert.Nil(t, s.EnsureService(22, "bar", &structs.NodeService{ID: "redis", Service: "redis", Tags: nil, Address: "", Port: 6379}))
		assert.True(t, watchFired(ws))

		ws = memdb.NewWatchSet()
		idx, out, err := s.DumpGatewayServices(ws)
		assert.Nil(t, err)
		assert.Equal(t, idx, uint64(22))
		assert.Len(t, out, 3)

		expect := structs.GatewayServices{
			{
				Service:     structs.NewServiceName("api", nil),
				Gateway:     structs.NewServiceName("gateway", nil),
				GatewayKind: structs.ServiceKindTerminatingGateway,
				CAFile:      "api/ca.crt",
				CertFile:    "api/client.crt",
				KeyFile:     "api/client.key",
				SNI:         "my-domain",
				RaftIndex: structs.RaftIndex{
					CreateIndex: 21,
					ModifyIndex: 21,
				},
				ServiceKind: structs.GatewayServiceKindService,
			},
			{
				Service:     structs.NewServiceName("db", nil),
				Gateway:     structs.NewServiceName("gateway", nil),
				GatewayKind: structs.ServiceKindTerminatingGateway,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 21,
					ModifyIndex: 21,
				},
				ServiceKind: structs.GatewayServiceKindService,
			},
			{
				Service:      structs.NewServiceName("redis", nil),
				Gateway:      structs.NewServiceName("gateway", nil),
				GatewayKind:  structs.ServiceKindTerminatingGateway,
				CAFile:       "ca.crt",
				CertFile:     "client.crt",
				KeyFile:      "client.key",
				SNI:          "my-alt-domain",
				FromWildcard: true,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 22,
					ModifyIndex: 22,
				},
				ServiceKind: structs.GatewayServiceKindService,
			},
		}
		assert.Equal(t, expect, out)
	})

	// Delete a service covered by wildcard
	t.Run("delete-wc-service", func(t *testing.T) {
		assert.Nil(t, s.DeleteService(23, "bar", "redis", nil, ""))
		assert.True(t, watchFired(ws))

		ws = memdb.NewWatchSet()
		idx, out, err := s.DumpGatewayServices(ws)
		assert.Nil(t, err)
		assert.Equal(t, idx, uint64(23))
		assert.Len(t, out, 2)

		expect := structs.GatewayServices{
			{
				Service:     structs.NewServiceName("api", nil),
				Gateway:     structs.NewServiceName("gateway", nil),
				GatewayKind: structs.ServiceKindTerminatingGateway,
				CAFile:      "api/ca.crt",
				CertFile:    "api/client.crt",
				KeyFile:     "api/client.key",
				SNI:         "my-domain",
				RaftIndex: structs.RaftIndex{
					CreateIndex: 21,
					ModifyIndex: 21,
				},
				ServiceKind: structs.GatewayServiceKindService,
			},
			{
				Service:     structs.NewServiceName("db", nil),
				Gateway:     structs.NewServiceName("gateway", nil),
				GatewayKind: structs.ServiceKindTerminatingGateway,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 21,
					ModifyIndex: 21,
				},
				ServiceKind: structs.GatewayServiceKindService,
			},
		}
		assert.Equal(t, expect, out)
	})

	t.Run("delete-config-entry-svc", func(t *testing.T) {
		// Update the entry that only leaves one service
		assert.Nil(t, s.EnsureConfigEntry(24, &structs.TerminatingGatewayConfigEntry{
			Kind: "terminating-gateway",
			Name: "gateway",
			Services: []structs.LinkedService{
				{
					Name: "db",
				},
			},
		}))
		assert.True(t, watchFired(ws))

		idx, out, err := s.DumpGatewayServices(ws)
		assert.Nil(t, err)
		assert.Equal(t, idx, uint64(24))
		assert.Len(t, out, 1)

		// previously associated service (api) should not be present
		expect := structs.GatewayServices{
			{
				Service:     structs.NewServiceName("db", nil),
				Gateway:     structs.NewServiceName("gateway", nil),
				GatewayKind: structs.ServiceKindTerminatingGateway,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 24,
					ModifyIndex: 24,
				},
				ServiceKind: structs.GatewayServiceKindService,
			},
		}
		assert.Equal(t, expect, out)
	})

	t.Run("add-ingress-config", func(t *testing.T) {
		svcDefault := &structs.ServiceConfigEntry{
			Name:     "web",
			Kind:     structs.ServiceDefaults,
			Protocol: "http",
		}
		assert.NoError(t, s.EnsureConfigEntry(25, svcDefault))

		// Associate gateway with db and api
		assert.Nil(t, s.EnsureConfigEntry(26, &structs.IngressGatewayConfigEntry{
			Kind: "ingress-gateway",
			Name: "ingress",
			Listeners: []structs.IngressListener{
				{
					Port:     1111,
					Protocol: "tcp",
					Services: []structs.IngressService{
						{
							Name: "api",
						},
					},
				},
				{
					Port:     2222,
					Protocol: "http",
					Services: []structs.IngressService{
						{
							Name:  "web",
							Hosts: []string{"web.example.com"},
						},
					},
				},
			},
		}))
		assert.True(t, watchFired(ws))

		// Read everything back.
		ws = memdb.NewWatchSet()
		idx, out, err := s.DumpGatewayServices(ws)
		assert.Nil(t, err)
		assert.Equal(t, idx, uint64(26))
		assert.Len(t, out, 3)

		expect := structs.GatewayServices{
			{
				Service:     structs.NewServiceName("db", nil),
				Gateway:     structs.NewServiceName("gateway", nil),
				GatewayKind: structs.ServiceKindTerminatingGateway,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 24,
					ModifyIndex: 24,
				},
				ServiceKind: structs.GatewayServiceKindService,
			},
			{
				Service:     structs.NewServiceName("api", nil),
				Gateway:     structs.NewServiceName("ingress", nil),
				GatewayKind: structs.ServiceKindIngressGateway,
				Protocol:    "tcp",
				Port:        1111,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 26,
					ModifyIndex: 26,
				},
			},
			{
				Service:     structs.NewServiceName("web", nil),
				Gateway:     structs.NewServiceName("ingress", nil),
				GatewayKind: structs.ServiceKindIngressGateway,
				Protocol:    "http",
				Port:        2222,
				Hosts:       []string{"web.example.com"},
				RaftIndex: structs.RaftIndex{
					CreateIndex: 26,
					ModifyIndex: 26,
				},
			},
		}
		assert.Equal(t, expect, out)
	})

	t.Run("delete-tgw-entry", func(t *testing.T) {
		// Deleting the config entry should remove existing mappings
		assert.Nil(t, s.DeleteConfigEntry(27, "terminating-gateway", "gateway", nil))
		assert.True(t, watchFired(ws))

		idx, out, err := s.DumpGatewayServices(ws)
		assert.Nil(t, err)
		assert.Equal(t, idx, uint64(27))
		assert.Len(t, out, 2)

		// Only ingress entries should remain
		expect := structs.GatewayServices{
			{
				Service:     structs.NewServiceName("api", nil),
				Gateway:     structs.NewServiceName("ingress", nil),
				GatewayKind: structs.ServiceKindIngressGateway,
				Protocol:    "tcp",
				Port:        1111,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 26,
					ModifyIndex: 26,
				},
			},
			{
				Service:     structs.NewServiceName("web", nil),
				Gateway:     structs.NewServiceName("ingress", nil),
				GatewayKind: structs.ServiceKindIngressGateway,
				Protocol:    "http",
				Port:        2222,
				Hosts:       []string{"web.example.com"},
				RaftIndex: structs.RaftIndex{
					CreateIndex: 26,
					ModifyIndex: 26,
				},
			},
		}
		assert.Equal(t, expect, out)
	})

	t.Run("delete-ingress-entry", func(t *testing.T) {
		// Deleting the config entry should remove existing mappings
		assert.Nil(t, s.DeleteConfigEntry(28, "ingress-gateway", "ingress", nil))
		assert.True(t, watchFired(ws))

		idx, out, err := s.DumpGatewayServices(ws)
		assert.Nil(t, err)
		assert.Equal(t, idx, uint64(28))
		assert.Len(t, out, 0)
	})
}

func TestCatalog_catalogDownstreams_Watches(t *testing.T) {
	type expect struct {
		idx   uint64
		names []structs.ServiceName
	}

	s := testStateStore(t)

	require.NoError(t, s.EnsureNode(0, &structs.Node{
		ID:   "c73b8fdf-4ef8-4e43-9aa2-59e85cc6a70c",
		Node: "foo",
	}))

	defaultMeta := structs.DefaultEnterpriseMetaInDefaultPartition()

	admin := structs.NewServiceName("admin", defaultMeta)
	cache := structs.NewServiceName("cache", defaultMeta)

	// Watch should fire since the admin <-> web-proxy pairing was inserted into the topology table
	ws := memdb.NewWatchSet()
	tx := s.db.ReadTxn()
	idx, names, err := downstreamsFromRegistrationTxn(tx, ws, admin)
	require.NoError(t, err)
	assert.Zero(t, idx)
	assert.Len(t, names, 0)

	svc := structs.NodeService{
		Kind:    structs.ServiceKindConnectProxy,
		ID:      "web-proxy",
		Service: "web-proxy",
		Address: "127.0.0.2",
		Port:    443,
		Proxy: structs.ConnectProxyConfig{
			DestinationServiceName: "web",
			Upstreams: structs.Upstreams{
				structs.Upstream{
					DestinationName: "db",
				},
				structs.Upstream{
					DestinationName: "admin",
				},
			},
		},
		EnterpriseMeta: *defaultMeta,
	}
	require.NoError(t, s.EnsureService(1, "foo", &svc))
	assert.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	tx = s.db.ReadTxn()
	idx, names, err = downstreamsFromRegistrationTxn(tx, ws, admin)
	require.NoError(t, err)

	exp := expect{
		idx: 1,
		names: []structs.ServiceName{
			{Name: "web", EnterpriseMeta: *defaultMeta},
		},
	}
	require.Equal(t, exp.idx, idx)
	require.ElementsMatch(t, exp.names, names)

	// Now replace the admin upstream to verify watch fires and mapping is removed
	svc.Proxy.Upstreams = structs.Upstreams{
		structs.Upstream{
			DestinationName: "db",
		},
		structs.Upstream{
			DestinationName: "not-admin",
		},
		structs.Upstream{
			DestinationName: "cache",
		},
	}
	require.NoError(t, s.EnsureService(2, "foo", &svc))
	assert.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	tx = s.db.ReadTxn()
	idx, _, err = downstreamsFromRegistrationTxn(tx, ws, admin)
	require.NoError(t, err)

	exp = expect{
		// Expect index where the upstream was replaced
		idx: 2,
	}
	require.Equal(t, exp.idx, idx)
	require.Empty(t, exp.names)

	// Should still be able to get downstream for one of the other upstreams
	ws = memdb.NewWatchSet()
	tx = s.db.ReadTxn()
	idx, names, err = downstreamsFromRegistrationTxn(tx, ws, cache)
	require.NoError(t, err)

	exp = expect{
		idx: 2,
		names: []structs.ServiceName{
			{Name: "web", EnterpriseMeta: *defaultMeta},
		},
	}
	require.Equal(t, exp.idx, idx)
	require.ElementsMatch(t, exp.names, names)

	// Now delete the web-proxy service and the result should be empty
	require.NoError(t, s.DeleteService(3, "foo", "web-proxy", defaultMeta, ""))
	assert.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	tx = s.db.ReadTxn()
	idx, _, err = downstreamsFromRegistrationTxn(tx, ws, cache)

	require.NoError(t, err)

	exp = expect{
		// Expect deletion index
		idx: 3,
	}
	require.Equal(t, exp.idx, idx)
	require.Empty(t, exp.names)
}

func TestCatalog_catalogDownstreams(t *testing.T) {
	defaultMeta := structs.DefaultEnterpriseMetaInDefaultPartition()

	type expect struct {
		idx   uint64
		names []structs.ServiceName
	}
	tt := []struct {
		name     string
		services []*structs.NodeService
		expect   expect
	}{
		{
			name: "single proxy with multiple upstreams",
			services: []*structs.NodeService{
				{
					Kind:    structs.ServiceKindConnectProxy,
					ID:      "api-proxy",
					Service: "api-proxy",
					Address: "127.0.0.1",
					Port:    443,
					Proxy: structs.ConnectProxyConfig{
						DestinationServiceName: "api",
						Upstreams: structs.Upstreams{
							structs.Upstream{
								DestinationName: "cache",
							},
							structs.Upstream{
								DestinationName: "db",
							},
							structs.Upstream{
								DestinationName: "admin",
							},
						},
					},
					EnterpriseMeta: *defaultMeta,
				},
			},
			expect: expect{
				idx: 1,
				names: []structs.ServiceName{
					{Name: "api", EnterpriseMeta: *defaultMeta},
				},
			},
		},
		{
			name: "multiple proxies with multiple upstreams",
			services: []*structs.NodeService{
				{
					Kind:    structs.ServiceKindConnectProxy,
					ID:      "api-proxy",
					Service: "api-proxy",
					Address: "127.0.0.1",
					Port:    443,
					Proxy: structs.ConnectProxyConfig{
						DestinationServiceName: "api",
						Upstreams: structs.Upstreams{
							structs.Upstream{
								DestinationName: "cache",
							},
							structs.Upstream{
								DestinationName: "db",
							},
							structs.Upstream{
								DestinationName: "admin",
							},
						},
					},
					EnterpriseMeta: *defaultMeta,
				},
				{
					Kind:    structs.ServiceKindConnectProxy,
					ID:      "web-proxy",
					Service: "web-proxy",
					Address: "127.0.0.2",
					Port:    443,
					Proxy: structs.ConnectProxyConfig{
						DestinationServiceName: "web",
						Upstreams: structs.Upstreams{
							structs.Upstream{
								DestinationName: "db",
							},
							structs.Upstream{
								DestinationName: "admin",
							},
						},
					},
					EnterpriseMeta: *defaultMeta,
				},
			},
			expect: expect{
				idx: 2,
				names: []structs.ServiceName{
					{Name: "api", EnterpriseMeta: *defaultMeta},
					{Name: "web", EnterpriseMeta: *defaultMeta},
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			s := testStateStore(t)
			ws := memdb.NewWatchSet()

			require.NoError(t, s.EnsureNode(0, &structs.Node{
				ID:   "c73b8fdf-4ef8-4e43-9aa2-59e85cc6a70c",
				Node: "foo",
			}))

			var i uint64 = 1
			for _, svc := range tc.services {
				require.NoError(t, s.EnsureService(i, "foo", svc))
				i++
			}

			tx := s.db.ReadTxn()
			idx, names, err := downstreamsFromRegistrationTxn(tx, ws, structs.NewServiceName("admin", structs.DefaultEnterpriseMetaInDefaultPartition()))
			require.NoError(t, err)

			require.Equal(t, tc.expect.idx, idx)
			require.ElementsMatch(t, tc.expect.names, names)
		})
	}
}

func TestCatalog_upstreamsFromRegistration(t *testing.T) {
	defaultMeta := structs.DefaultEnterpriseMetaInDefaultPartition()

	type expect struct {
		idx   uint64
		names []structs.ServiceName
	}
	tt := []struct {
		name     string
		services []*structs.NodeService
		expect   expect
	}{
		{
			name: "single proxy with multiple upstreams",
			services: []*structs.NodeService{
				{
					Kind:    structs.ServiceKindConnectProxy,
					ID:      "api-proxy",
					Service: "api-proxy",
					Address: "127.0.0.1",
					Port:    443,
					Proxy: structs.ConnectProxyConfig{
						DestinationServiceName: "api",
						Upstreams: structs.Upstreams{
							structs.Upstream{
								DestinationName: "cache",
							},
							structs.Upstream{
								DestinationName: "db",
							},
							structs.Upstream{
								DestinationName: "admin",
							},
						},
					},
					EnterpriseMeta: *defaultMeta,
				},
			},
			expect: expect{
				idx: 1,
				names: []structs.ServiceName{
					{Name: "cache", EnterpriseMeta: *defaultMeta},
					{Name: "db", EnterpriseMeta: *defaultMeta},
					{Name: "admin", EnterpriseMeta: *defaultMeta},
				},
			},
		},
		{
			name: "multiple proxies with multiple upstreams",
			services: []*structs.NodeService{
				{
					Kind:    structs.ServiceKindConnectProxy,
					ID:      "api-proxy",
					Service: "api-proxy",
					Address: "127.0.0.1",
					Port:    443,
					Proxy: structs.ConnectProxyConfig{
						DestinationServiceName: "api",
						Upstreams: structs.Upstreams{
							structs.Upstream{
								DestinationName: "cache",
							},
							structs.Upstream{
								DestinationName: "db",
							},
							structs.Upstream{
								DestinationName: "admin",
							},
						},
					},
					EnterpriseMeta: *defaultMeta,
				},
				{
					Kind:    structs.ServiceKindConnectProxy,
					ID:      "api-proxy-2",
					Service: "api-proxy",
					Address: "127.0.0.2",
					Port:    443,
					Proxy: structs.ConnectProxyConfig{
						DestinationServiceName: "api",
						Upstreams: structs.Upstreams{
							structs.Upstream{
								DestinationName: "cache",
							},
							structs.Upstream{
								DestinationName: "db",
							},
							structs.Upstream{
								DestinationName: "new-admin",
							},
						},
					},
					EnterpriseMeta: *defaultMeta,
				},
				{
					Kind:    structs.ServiceKindConnectProxy,
					ID:      "different-api-proxy",
					Service: "different-api-proxy",
					Address: "127.0.0.4",
					Port:    443,
					Proxy: structs.ConnectProxyConfig{
						DestinationServiceName: "api",
						Upstreams: structs.Upstreams{
							structs.Upstream{
								DestinationName: "elasticache",
							},
							structs.Upstream{
								DestinationName: "db",
							},
							structs.Upstream{
								DestinationName: "admin",
							},
						},
					},
					EnterpriseMeta: *defaultMeta,
				},
				{
					Kind:    structs.ServiceKindConnectProxy,
					ID:      "web-proxy",
					Service: "web-proxy",
					Address: "127.0.0.3",
					Port:    80,
					Proxy: structs.ConnectProxyConfig{
						DestinationServiceName: "web",
						Upstreams: structs.Upstreams{
							structs.Upstream{
								DestinationName: "db",
							},
							structs.Upstream{
								DestinationName: "billing",
							},
						},
					},
					EnterpriseMeta: *defaultMeta,
				},
			},
			expect: expect{
				idx: 4,
				names: []structs.ServiceName{
					{Name: "cache", EnterpriseMeta: *defaultMeta},
					{Name: "db", EnterpriseMeta: *defaultMeta},
					{Name: "admin", EnterpriseMeta: *defaultMeta},
					{Name: "new-admin", EnterpriseMeta: *defaultMeta},
					{Name: "elasticache", EnterpriseMeta: *defaultMeta},
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			s := testStateStore(t)
			ws := memdb.NewWatchSet()

			require.NoError(t, s.EnsureNode(0, &structs.Node{
				ID:   "c73b8fdf-4ef8-4e43-9aa2-59e85cc6a70c",
				Node: "foo",
			}))

			var i uint64 = 1
			for _, svc := range tc.services {
				require.NoError(t, s.EnsureService(i, "foo", svc))
				i++
			}

			tx := s.db.ReadTxn()
			idx, names, err := upstreamsFromRegistrationTxn(tx, ws, structs.NewServiceName("api", structs.DefaultEnterpriseMetaInDefaultPartition()))
			require.NoError(t, err)

			require.Equal(t, tc.expect.idx, idx)
			require.ElementsMatch(t, tc.expect.names, names)
		})
	}
}

func TestCatalog_upstreamsFromRegistration_Watches(t *testing.T) {
	type expect struct {
		idx   uint64
		names []structs.ServiceName
	}

	s := testStateStore(t)

	require.NoError(t, s.EnsureNode(0, &structs.Node{
		ID:   "c73b8fdf-4ef8-4e43-9aa2-59e85cc6a70c",
		Node: "foo",
	}))

	defaultMeta := structs.DefaultEnterpriseMetaInDefaultPartition()
	web := structs.NewServiceName("web", defaultMeta)

	ws := memdb.NewWatchSet()
	tx := s.db.ReadTxn()
	idx, names, err := upstreamsFromRegistrationTxn(tx, ws, web)
	require.NoError(t, err)
	assert.Zero(t, idx)
	assert.Len(t, names, 0)

	// Watch should fire since the admin <-> web pairing was inserted into the topology table
	svc := structs.NodeService{
		Kind:    structs.ServiceKindConnectProxy,
		ID:      "web-proxy",
		Service: "web-proxy",
		Address: "127.0.0.2",
		Port:    443,
		Proxy: structs.ConnectProxyConfig{
			DestinationServiceName: "web",
			Upstreams: structs.Upstreams{
				structs.Upstream{
					DestinationName: "db",
				},
				structs.Upstream{
					DestinationName: "admin",
				},
			},
		},
		EnterpriseMeta: *defaultMeta,
	}
	require.NoError(t, s.EnsureService(1, "foo", &svc))
	assert.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	tx = s.db.ReadTxn()
	idx, names, err = upstreamsFromRegistrationTxn(tx, ws, web)
	require.NoError(t, err)

	exp := expect{
		idx: 1,
		names: []structs.ServiceName{
			{Name: "db", EnterpriseMeta: *defaultMeta},
			{Name: "admin", EnterpriseMeta: *defaultMeta},
		},
	}
	require.Equal(t, exp.idx, idx)
	require.ElementsMatch(t, exp.names, names)

	// Now edit the upstreams list to verify watch fires and mapping is removed
	svc.Proxy.Upstreams = structs.Upstreams{
		structs.Upstream{
			DestinationName: "db",
		},
		structs.Upstream{
			DestinationName: "not-admin",
		},
	}
	require.NoError(t, s.EnsureService(2, "foo", &svc))
	assert.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	tx = s.db.ReadTxn()
	idx, names, err = upstreamsFromRegistrationTxn(tx, ws, web)
	require.NoError(t, err)

	exp = expect{
		// Expect index where the upstream was replaced
		idx: 2,
		names: []structs.ServiceName{
			{Name: "db", EnterpriseMeta: *defaultMeta},
			{Name: "not-admin", EnterpriseMeta: *defaultMeta},
		},
	}
	require.Equal(t, exp.idx, idx)
	require.ElementsMatch(t, exp.names, names)

	// Adding a new instance with distinct upstreams should result in a list that joins both
	svc = structs.NodeService{
		Kind:    structs.ServiceKindConnectProxy,
		ID:      "web-proxy-2",
		Service: "web-proxy",
		Address: "127.0.0.3",
		Port:    443,
		Proxy: structs.ConnectProxyConfig{
			DestinationServiceName: "web",
			Upstreams: structs.Upstreams{
				structs.Upstream{
					DestinationName: "db",
				},
				structs.Upstream{
					DestinationName: "also-not-admin",
				},
				structs.Upstream{
					DestinationName: "cache",
				},
			},
		},
		EnterpriseMeta: *defaultMeta,
	}
	require.NoError(t, s.EnsureService(3, "foo", &svc))
	assert.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	tx = s.db.ReadTxn()
	idx, names, err = upstreamsFromRegistrationTxn(tx, ws, web)
	require.NoError(t, err)

	exp = expect{
		idx: 3,
		names: []structs.ServiceName{
			{Name: "db", EnterpriseMeta: *defaultMeta},
			{Name: "not-admin", EnterpriseMeta: *defaultMeta},
			{Name: "also-not-admin", EnterpriseMeta: *defaultMeta},
			{Name: "cache", EnterpriseMeta: *defaultMeta},
		},
	}
	require.Equal(t, exp.idx, idx)
	require.ElementsMatch(t, exp.names, names)

	// Now delete the web-proxy service and the result should mirror the one of the remaining instance
	require.NoError(t, s.DeleteService(4, "foo", "web-proxy", defaultMeta, ""))
	assert.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	tx = s.db.ReadTxn()
	idx, names, err = upstreamsFromRegistrationTxn(tx, ws, web)
	require.NoError(t, err)

	exp = expect{
		idx: 4,
		names: []structs.ServiceName{
			{Name: "db", EnterpriseMeta: *defaultMeta},
			{Name: "also-not-admin", EnterpriseMeta: *defaultMeta},
			{Name: "cache", EnterpriseMeta: *defaultMeta},
		},
	}
	require.Equal(t, exp.idx, idx)
	require.ElementsMatch(t, exp.names, names)

	// Now delete the last web-proxy instance and the mappings should be cleared
	require.NoError(t, s.DeleteService(5, "foo", "web-proxy-2", defaultMeta, ""))
	assert.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	tx = s.db.ReadTxn()
	idx, names, err = upstreamsFromRegistrationTxn(tx, ws, web)

	require.NoError(t, err)

	exp = expect{
		// Expect deletion index
		idx: 5,
	}
	require.Equal(t, exp.idx, idx)
	require.Equal(t, exp.names, names)
}

func TestCatalog_topologyCleanupPanic(t *testing.T) {
	s := testStateStore(t)

	require.NoError(t, s.EnsureNode(0, &structs.Node{
		ID:   "c73b8fdf-4ef8-4e43-9aa2-59e85cc6a70c",
		Node: "foo",
	}))

	defaultMeta := structs.DefaultEnterpriseMetaInDefaultPartition()
	web := structs.NewServiceName("web", defaultMeta)

	ws := memdb.NewWatchSet()
	tx := s.db.ReadTxn()
	idx, names, err := upstreamsFromRegistrationTxn(tx, ws, web)
	require.NoError(t, err)
	assert.Zero(t, idx)
	assert.Len(t, names, 0)

	svc := structs.NodeService{
		Kind:    structs.ServiceKindConnectProxy,
		ID:      "web-proxy-1",
		Service: "web-proxy",
		Address: "127.0.0.2",
		Port:    443,
		Proxy: structs.ConnectProxyConfig{
			DestinationServiceName: "web",
			Upstreams: structs.Upstreams{
				structs.Upstream{
					DestinationName: "db",
				},
			},
		},
		EnterpriseMeta: *defaultMeta,
	}
	require.NoError(t, s.EnsureService(1, "foo", &svc))
	assert.True(t, watchFired(ws))

	svc = structs.NodeService{
		Kind:    structs.ServiceKindConnectProxy,
		ID:      "web-proxy-2",
		Service: "web-proxy",
		Address: "127.0.0.2",
		Port:    443,
		Proxy: structs.ConnectProxyConfig{
			DestinationServiceName: "web",
			Upstreams: structs.Upstreams{
				structs.Upstream{
					DestinationName: "db",
				},
				structs.Upstream{
					DestinationName: "cache",
				},
			},
		},
		EnterpriseMeta: *defaultMeta,
	}
	require.NoError(t, s.EnsureService(2, "foo", &svc))
	assert.True(t, watchFired(ws))

	// Now delete the node Foo, and this would panic because of the deletion within an iterator
	require.NoError(t, s.DeleteNode(3, "foo", nil, ""))
	assert.True(t, watchFired(ws))

}

func TestCatalog_upstreamsFromRegistration_Ingress(t *testing.T) {
	type expect struct {
		idx   uint64
		names []structs.ServiceName
	}

	s := testStateStore(t)

	require.NoError(t, s.EnsureNode(0, &structs.Node{
		ID:   "c73b8fdf-4ef8-4e43-9aa2-59e85cc6a70c",
		Node: "foo",
	}))
	require.NoError(t, s.EnsureConfigEntry(1, &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: structs.ProxyConfigGlobal,
		Config: map[string]interface{}{
			"protocol": "http",
		},
	}))

	defaultMeta := structs.DefaultEnterpriseMetaInDefaultPartition()
	ingress := structs.NewServiceName("ingress", defaultMeta)

	ws := memdb.NewWatchSet()
	tx := s.db.ReadTxn()
	idx, names, err := upstreamsFromRegistrationTxn(tx, ws, ingress)
	require.NoError(t, err)
	assert.Zero(t, idx)
	assert.Len(t, names, 0)

	// Watch should fire since the ingress -> [web, api] mappings were inserted into the topology table
	require.NoError(t, s.EnsureConfigEntry(2, &structs.IngressGatewayConfigEntry{
		Kind: "ingress-gateway",
		Name: "ingress",
		Listeners: []structs.IngressListener{
			{
				Port:     1111,
				Protocol: "http",
				Services: []structs.IngressService{
					{
						Name:           "api",
						EnterpriseMeta: *defaultMeta,
					},
					{
						Name:           "web",
						EnterpriseMeta: *defaultMeta,
					},
				},
			},
		},
	}))
	assert.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	tx = s.db.ReadTxn()

	idx, names, err = upstreamsFromRegistrationTxn(tx, ws, ingress)
	require.NoError(t, err)

	exp := expect{
		idx: 2,
		names: []structs.ServiceName{
			{Name: "api", EnterpriseMeta: *defaultMeta},
			{Name: "web", EnterpriseMeta: *defaultMeta},
		},
	}
	require.Equal(t, exp.idx, idx)
	require.ElementsMatch(t, exp.names, names)

	// Now delete a gateway service and topology table should be updated
	require.NoError(t, s.EnsureConfigEntry(3, &structs.IngressGatewayConfigEntry{
		Kind: "ingress-gateway",
		Name: "ingress",
		Listeners: []structs.IngressListener{
			{
				Port:     1111,
				Protocol: "http",
				Services: []structs.IngressService{
					{
						Name:           "api",
						EnterpriseMeta: *defaultMeta,
					},
				},
			},
		},
	}))
	assert.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	tx = s.db.ReadTxn()
	idx, names, err = upstreamsFromRegistrationTxn(tx, ws, ingress)
	require.NoError(t, err)

	exp = expect{
		// Expect index where the upstream was replaced
		idx: 3,
		names: []structs.ServiceName{
			{Name: "api", EnterpriseMeta: *defaultMeta},
		},
	}
	require.Equal(t, exp.idx, idx)
	require.ElementsMatch(t, exp.names, names)

	// Now replace api with a wildcard and no services should be returned because none are registered
	require.NoError(t, s.EnsureConfigEntry(4, &structs.IngressGatewayConfigEntry{
		Kind: "ingress-gateway",
		Name: "ingress",
		Listeners: []structs.IngressListener{
			{
				Port:     1111,
				Protocol: "http",
				Services: []structs.IngressService{
					{
						Name:           "*",
						EnterpriseMeta: *defaultMeta,
					},
				},
			},
		},
	}))
	assert.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	tx = s.db.ReadTxn()
	idx, names, err = upstreamsFromRegistrationTxn(tx, ws, ingress)
	require.NoError(t, err)
	require.Equal(t, uint64(4), idx)
	require.Len(t, names, 0)

	// Adding a service will be covered by the ingress wildcard and added to the topology
	svc := structs.NodeService{
		ID:             "db",
		Service:        "db",
		Address:        "127.0.0.3",
		Port:           443,
		EnterpriseMeta: *defaultMeta,
		Connect:        structs.ServiceConnect{Native: true},
	}
	require.NoError(t, s.EnsureService(5, "foo", &svc))
	assert.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	tx = s.db.ReadTxn()
	idx, names, err = upstreamsFromRegistrationTxn(tx, ws, ingress)
	require.NoError(t, err)

	exp = expect{
		// Expect index where the upstream was replaced
		idx: 5,
		names: []structs.ServiceName{
			{Name: "db", EnterpriseMeta: *defaultMeta},
		},
	}
	require.Equal(t, exp.idx, idx)
	require.ElementsMatch(t, exp.names, names)

	// Deleting a service covered by a wildcard should delete its mapping
	require.NoError(t, s.DeleteService(6, "foo", svc.ID, &svc.EnterpriseMeta, ""))
	assert.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	tx = s.db.ReadTxn()
	idx, names, err = upstreamsFromRegistrationTxn(tx, ws, ingress)
	require.NoError(t, err)
	require.Equal(t, uint64(6), idx)
	require.Len(t, names, 0)

	// Now add a service again, to test the effect of deleting the config entry itself
	require.NoError(t, s.EnsureConfigEntry(7, &structs.IngressGatewayConfigEntry{
		Kind: "ingress-gateway",
		Name: "ingress",
		Listeners: []structs.IngressListener{
			{
				Port:     1111,
				Protocol: "http",
				Services: []structs.IngressService{
					{
						Name:           "api",
						EnterpriseMeta: *defaultMeta,
					},
				},
			},
		},
	}))
	assert.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	tx = s.db.ReadTxn()
	idx, names, err = upstreamsFromRegistrationTxn(tx, ws, ingress)
	require.NoError(t, err)

	exp = expect{
		// Expect index where the upstream was replaced
		idx: 7,
		names: []structs.ServiceName{
			{Name: "api", EnterpriseMeta: *defaultMeta},
		},
	}
	require.Equal(t, exp.idx, idx)
	require.ElementsMatch(t, exp.names, names)

	// Deleting the config entry should remove the mapping
	require.NoError(t, s.DeleteConfigEntry(8, "ingress-gateway", "ingress", defaultMeta))
	assert.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	tx = s.db.ReadTxn()
	idx, names, err = upstreamsFromRegistrationTxn(tx, ws, ingress)
	require.NoError(t, err)
	require.Equal(t, uint64(8), idx)
	require.Len(t, names, 0)
}

func TestCatalog_cleanupGatewayWildcards_panic(t *testing.T) {
	s := testStateStore(t)

	require.NoError(t, s.EnsureNode(0, &structs.Node{
		ID:   "c73b8fdf-4ef8-4e43-9aa2-59e85cc6a70c",
		Node: "foo",
	}))
	require.NoError(t, s.EnsureConfigEntry(1, &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: structs.ProxyConfigGlobal,
		Config: map[string]interface{}{
			"protocol": "http",
		},
	}))

	defaultMeta := structs.DefaultEnterpriseMetaInDefaultPartition()

	// Register two different gateways that target services via wildcard
	require.NoError(t, s.EnsureConfigEntry(2, &structs.TerminatingGatewayConfigEntry{
		Kind: "terminating-gateway",
		Name: "my-gateway-1-terminating",
		Services: []structs.LinkedService{
			{
				Name:           "*",
				EnterpriseMeta: *defaultMeta,
			},
		},
	}))

	require.NoError(t, s.EnsureConfigEntry(3, &structs.IngressGatewayConfigEntry{
		Kind: "ingress-gateway",
		Name: "my-gateway-2-ingress",
		Listeners: []structs.IngressListener{
			{
				Port:     1111,
				Protocol: "http",
				Services: []structs.IngressService{
					{
						Name:           "*",
						EnterpriseMeta: *defaultMeta,
					},
				},
			},
		},
	}))

	// Register two services that share a prefix, both will be covered by gateway wildcards above
	api := structs.NodeService{
		ID:             "api",
		Service:        "api",
		Address:        "127.0.0.2",
		Port:           443,
		EnterpriseMeta: *defaultMeta,
	}
	require.NoError(t, s.EnsureService(4, "foo", &api))

	api2 := structs.NodeService{
		ID:             "api-2",
		Service:        "api-2",
		Address:        "127.0.0.2",
		Port:           443,
		EnterpriseMeta: *defaultMeta,
	}
	require.NoError(t, s.EnsureService(5, "foo", &api2))

	// Now delete the node "foo", and this would panic because of the deletion within an iterator
	require.NoError(t, s.DeleteNode(6, "foo", nil, ""))
}

func TestCatalog_DownstreamsForService(t *testing.T) {
	defaultMeta := structs.DefaultEnterpriseMetaInDefaultPartition()

	type expect struct {
		idx   uint64
		names []structs.ServiceName
	}
	tt := []struct {
		name     string
		services []*structs.NodeService
		entries  []structs.ConfigEntry
		expect   expect
	}{
		{
			name: "kitchen sink",
			services: []*structs.NodeService{
				{
					Kind:    structs.ServiceKindConnectProxy,
					ID:      "api-proxy",
					Service: "api-proxy",
					Address: "127.0.0.1",
					Port:    443,
					Proxy: structs.ConnectProxyConfig{
						DestinationServiceName: "api",
						Upstreams: structs.Upstreams{
							structs.Upstream{
								DestinationName: "cache",
							},
							structs.Upstream{
								DestinationName: "db",
							},
							structs.Upstream{
								DestinationName: "old-admin",
							},
						},
					},
					EnterpriseMeta: *defaultMeta,
				},
				{
					Kind:    structs.ServiceKindConnectProxy,
					ID:      "web-proxy",
					Service: "web-proxy",
					Address: "127.0.0.2",
					Port:    443,
					Proxy: structs.ConnectProxyConfig{
						DestinationServiceName: "web",
						Upstreams: structs.Upstreams{
							structs.Upstream{
								DestinationName: "db",
							},
							structs.Upstream{
								DestinationName: "admin",
							},
						},
					},
					EnterpriseMeta: *defaultMeta,
				},
			},
			entries: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Config: map[string]interface{}{
						"protocol": "http",
					},
				},
				&structs.ServiceRouterConfigEntry{
					Kind: structs.ServiceRouter,
					Name: "old-admin",
					Routes: []structs.ServiceRoute{
						{
							Match: &structs.ServiceRouteMatch{
								HTTP: &structs.ServiceRouteHTTPMatch{
									PathExact: "/v2",
								},
							},
							Destination: &structs.ServiceRouteDestination{
								Service: "admin",
							},
						},
					},
				},
			},
			expect: expect{
				idx: 4,
				names: []structs.ServiceName{
					// get web from listing admin directly as an upstream
					{Name: "web", EnterpriseMeta: *defaultMeta},
					// get api from old-admin routing to admin and web listing old-admin as an upstream
					{Name: "api", EnterpriseMeta: *defaultMeta},
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			s := testStateStore(t)

			require.NoError(t, s.EnsureNode(0, &structs.Node{
				ID:   "c73b8fdf-4ef8-4e43-9aa2-59e85cc6a70c",
				Node: "foo",
			}))

			var i uint64 = 1
			for _, svc := range tc.services {
				require.NoError(t, s.EnsureService(i, "foo", svc))
				i++
			}

			ca := &structs.CAConfiguration{
				Provider: "consul",
			}
			err := s.CASetConfig(0, ca)
			require.NoError(t, err)

			for _, entry := range tc.entries {
				require.NoError(t, entry.Normalize())
				require.NoError(t, s.EnsureConfigEntry(i, entry))
				i++
			}

			tx := s.db.ReadTxn()
			defer tx.Abort()

			ws := memdb.NewWatchSet()
			sn := structs.NewServiceName("admin", structs.DefaultEnterpriseMetaInDefaultPartition())
			idx, names, err := s.downstreamsForServiceTxn(tx, ws, "dc1", sn)
			require.NoError(t, err)

			require.Equal(t, tc.expect.idx, idx)
			require.ElementsMatch(t, tc.expect.names, names)
		})
	}
}

func TestCatalog_DownstreamsForService_Updates(t *testing.T) {
	var (
		defaultMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
		target      = structs.NewServiceName("admin", defaultMeta)
	)

	s := testStateStore(t)
	ca := &structs.CAConfiguration{
		Provider: "consul",
	}
	err := s.CASetConfig(1, ca)
	require.NoError(t, err)

	require.NoError(t, s.EnsureNode(2, &structs.Node{
		ID:   "c73b8fdf-4ef8-4e43-9aa2-59e85cc6a70c",
		Node: "foo",
	}))

	// Register a service with our target as an upstream, and it should show up as a downstream
	web := structs.NodeService{
		Kind:    structs.ServiceKindConnectProxy,
		ID:      "web-proxy",
		Service: "web-proxy",
		Address: "127.0.0.2",
		Port:    443,
		Proxy: structs.ConnectProxyConfig{
			DestinationServiceName: "web",
			Upstreams: structs.Upstreams{
				structs.Upstream{
					DestinationName: "db",
				},
				structs.Upstream{
					DestinationName: "admin",
				},
			},
		},
		EnterpriseMeta: *defaultMeta,
	}
	require.NoError(t, s.EnsureService(3, "foo", &web))

	ws := memdb.NewWatchSet()
	tx := s.db.ReadTxn()
	idx, names, err := s.downstreamsForServiceTxn(tx, ws, "dc1", target)
	require.NoError(t, err)
	tx.Abort()

	expect := []structs.ServiceName{
		{Name: "web", EnterpriseMeta: *defaultMeta},
	}
	require.Equal(t, uint64(3), idx)
	require.ElementsMatch(t, expect, names)

	// Register a service WITHOUT our target as an upstream, and the watch should not fire
	api := structs.NodeService{
		Kind:    structs.ServiceKindConnectProxy,
		ID:      "api-proxy",
		Service: "api-proxy",
		Address: "127.0.0.1",
		Port:    443,
		Proxy: structs.ConnectProxyConfig{
			DestinationServiceName: "api",
			Upstreams: structs.Upstreams{
				structs.Upstream{
					DestinationName: "cache",
				},
				structs.Upstream{
					DestinationName: "db",
				},
				structs.Upstream{
					DestinationName: "old-admin",
				},
			},
		},
		EnterpriseMeta: *defaultMeta,
	}
	require.NoError(t, s.EnsureService(4, "foo", &api))
	require.False(t, watchFired(ws))

	// Update the routing so that api's upstream routes to our target and watches should fire
	defaults := structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: structs.ProxyConfigGlobal,
		Config: map[string]interface{}{
			"protocol": "http",
		},
	}
	require.NoError(t, defaults.Normalize())
	require.NoError(t, s.EnsureConfigEntry(5, &defaults))

	router := structs.ServiceRouterConfigEntry{
		Kind: structs.ServiceRouter,
		Name: "old-admin",
		Routes: []structs.ServiceRoute{
			{
				Match: &structs.ServiceRouteMatch{
					HTTP: &structs.ServiceRouteHTTPMatch{
						PathExact: "/v2",
					},
				},
				Destination: &structs.ServiceRouteDestination{
					Service: "admin",
				},
			},
		},
	}
	require.NoError(t, router.Normalize())
	require.NoError(t, s.EnsureConfigEntry(6, &router))

	// We updated a relevant config entry
	require.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	tx = s.db.ReadTxn()
	idx, names, err = s.downstreamsForServiceTxn(tx, ws, "dc1", target)
	require.NoError(t, err)
	tx.Abort()

	expect = []structs.ServiceName{
		// get web from listing admin directly as an upstream
		{Name: "web", EnterpriseMeta: *defaultMeta},
		// get api from old-admin routing to admin and web listing old-admin as an upstream
		{Name: "api", EnterpriseMeta: *defaultMeta},
	}
	require.Equal(t, uint64(6), idx)
	require.ElementsMatch(t, expect, names)
}

func TestProtocolForIngressGateway(t *testing.T) {
	tt := []struct {
		name    string
		idx     uint64
		entries []structs.ConfigEntry
		expect  string
	}{
		{
			name: "all http like",
			idx:  uint64(5),
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "h1-svc",
					Protocol: "http",
				},
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "h2-svc",
					Protocol: "http2",
				},
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "g-svc",
					Protocol: "grpc",
				},
				&structs.IngressGatewayConfigEntry{
					Kind: structs.IngressGateway,
					Name: "ingress",
					Listeners: []structs.IngressListener{
						{
							Port:     1111,
							Protocol: "http",
							Services: []structs.IngressService{
								{
									Name: "h1-svc",
								},
							},
						},
						{
							Port:     2222,
							Protocol: "http2",
							Services: []structs.IngressService{
								{
									Name: "h2-svc",
								},
							},
						},
						{
							Port:     3333,
							Protocol: "grpc",
							Services: []structs.IngressService{
								{
									Name: "g-svc",
								},
							},
						},
					},
				},
			},
			expect: "http",
		},
		{
			name: "all tcp",
			idx:  uint64(6),
			entries: []structs.ConfigEntry{
				&structs.IngressGatewayConfigEntry{
					Kind: structs.IngressGateway,
					Name: "ingress",
					Listeners: []structs.IngressListener{
						{
							Port:     1111,
							Protocol: "tcp",
							Services: []structs.IngressService{
								{
									Name: "zip",
								},
							},
						},
						{
							Port:     2222,
							Protocol: "tcp",
							Services: []structs.IngressService{
								{
									Name: "zop",
								},
							},
						},
						{
							Port:     3333,
							Protocol: "tcp",
							Services: []structs.IngressService{
								{
									Name: "zap",
								},
							},
						},
					},
				},
			},
			expect: "tcp",
		},
		{
			name: "mix of both",
			idx:  uint64(7),
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "h1-svc",
					Protocol: "http",
				},
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "g-svc",
					Protocol: "grpc",
				},
				&structs.IngressGatewayConfigEntry{
					Kind: structs.IngressGateway,
					Name: "ingress",
					Listeners: []structs.IngressListener{
						{
							Port:     1111,
							Protocol: "http",
							Services: []structs.IngressService{
								{
									Name: "h1-svc",
								},
							},
						},
						{
							Port:     2222,
							Protocol: "tcp",
							Services: []structs.IngressService{
								{
									Name: "zop",
								},
							},
						},
						{
							Port:     3333,
							Protocol: "grpc",
							Services: []structs.IngressService{
								{
									Name: "g-svc",
								},
							},
						},
					},
				},
			},
			expect: "tcp",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			s := testStateStore(t)

			for _, entry := range tc.entries {
				require.NoError(t, entry.Normalize())
				require.NoError(t, entry.Validate())

				require.NoError(t, s.EnsureConfigEntry(tc.idx, entry))
			}

			tx := s.db.ReadTxn()
			defer tx.Abort()

			ws := memdb.NewWatchSet()
			sn := structs.NewServiceName("ingress", structs.DefaultEnterpriseMetaInDefaultPartition())

			idx, protocol, err := metricsProtocolForIngressGateway(tx, ws, sn)
			require.NoError(t, err)
			require.Equal(t, tc.idx, idx)
			require.Equal(t, tc.expect, protocol)
		})
	}
}

func TestStateStore_EnsureService_ServiceNames(t *testing.T) {
	s := testStateStore(t)

	// Create the service registration.
	entMeta := structs.DefaultEnterpriseMetaInDefaultPartition()

	services := []structs.NodeService{
		{
			Kind:           structs.ServiceKindIngressGateway,
			ID:             "ingress-gateway",
			Service:        "ingress-gateway",
			Address:        "2.2.2.2",
			Port:           2222,
			EnterpriseMeta: *entMeta,
		},
		{
			Kind:           structs.ServiceKindMeshGateway,
			ID:             "mesh-gateway",
			Service:        "mesh-gateway",
			Address:        "4.4.4.4",
			Port:           4444,
			EnterpriseMeta: *entMeta,
		},
		{
			Kind:           structs.ServiceKindConnectProxy,
			ID:             "connect-proxy",
			Service:        "connect-proxy",
			Address:        "1.1.1.1",
			Port:           1111,
			Proxy:          structs.ConnectProxyConfig{DestinationServiceName: "foo"},
			EnterpriseMeta: *entMeta,
		},
		{
			Kind:           structs.ServiceKindTerminatingGateway,
			ID:             "terminating-gateway",
			Service:        "terminating-gateway",
			Address:        "3.3.3.3",
			Port:           3333,
			EnterpriseMeta: *entMeta,
		},
		{
			Kind:           structs.ServiceKindTypical,
			ID:             "web",
			Service:        "web",
			Address:        "5.5.5.5",
			Port:           5555,
			EnterpriseMeta: *entMeta,
		},
	}

	var idx, connectEnabledIdx uint64
	testRegisterNode(t, s, idx, "node1")

	for _, svc := range services {
		idx++
		require.NoError(t, s.EnsureService(idx, "node1", &svc))

		// Ensure the service name was stored for all of them under the appropriate kind
		gotIdx, gotNames, err := s.ServiceNamesOfKind(nil, svc.Kind)
		require.NoError(t, err)
		require.Equal(t, idx, gotIdx)
		require.Len(t, gotNames, 1)
		require.Equal(t, svc.CompoundServiceName(), gotNames[0].Service)
		require.Equal(t, svc.Kind, gotNames[0].Kind)
		if svc.Kind == structs.ServiceKindConnectProxy {
			connectEnabledIdx = idx
		}
	}

	// A ConnectEnabled service should exist if a corresponding ConnectProxy or ConnectNative service exists.
	verifyConnectEnabled := func(expectIdx uint64) {
		gotIdx, gotNames, err := s.ServiceNamesOfKind(nil, structs.ServiceKindConnectEnabled)
		require.NoError(t, err)
		require.Equal(t, expectIdx, gotIdx)
		require.Equal(t, []*KindServiceName{
			{
				Kind:    structs.ServiceKindConnectEnabled,
				Service: structs.NewServiceName("foo", entMeta),
				RaftIndex: structs.RaftIndex{
					CreateIndex: connectEnabledIdx,
					ModifyIndex: connectEnabledIdx,
				},
			},
		}, gotNames)
	}
	verifyConnectEnabled(connectEnabledIdx)

	// Register another ingress gateway and there should be two names under the kind index
	newIngress := structs.NodeService{
		Kind:           structs.ServiceKindIngressGateway,
		ID:             "new-ingress-gateway",
		Service:        "new-ingress-gateway",
		Address:        "6.6.6.6",
		Port:           6666,
		EnterpriseMeta: *entMeta,
	}
	idx++
	require.NoError(t, s.EnsureService(idx, "node1", &newIngress))

	gotIdx, got, err := s.ServiceNamesOfKind(nil, structs.ServiceKindIngressGateway)
	require.NoError(t, err)
	require.Equal(t, idx, gotIdx)

	expect := []*KindServiceName{
		{
			Kind:    structs.ServiceKindIngressGateway,
			Service: structs.NewServiceName("ingress-gateway", nil),
			RaftIndex: structs.RaftIndex{
				CreateIndex: 1,
				ModifyIndex: 1,
			},
		},
		{
			Kind:    structs.ServiceKindIngressGateway,
			Service: structs.NewServiceName("new-ingress-gateway", nil),
			RaftIndex: structs.RaftIndex{
				CreateIndex: idx,
				ModifyIndex: idx,
			},
		},
	}
	require.Equal(t, expect, got)

	// Deregister an ingress gateway and the index should not slide back
	idx++
	require.NoError(t, s.DeleteService(idx, "node1", "new-ingress-gateway", entMeta, ""))

	gotIdx, got, err = s.ServiceNamesOfKind(nil, structs.ServiceKindIngressGateway)
	require.NoError(t, err)
	require.Equal(t, idx, gotIdx)
	require.Equal(t, expect[:1], got)

	// Registering another instance of a known service should not bump the kind index
	newMGW := structs.NodeService{
		Kind:           structs.ServiceKindMeshGateway,
		ID:             "mesh-gateway-1",
		Service:        "mesh-gateway",
		Address:        "7.7.7.7",
		Port:           7777,
		EnterpriseMeta: *entMeta,
	}
	idx++
	require.NoError(t, s.EnsureService(idx, "node1", &newMGW))

	gotIdx, _, err = s.ServiceNamesOfKind(nil, structs.ServiceKindMeshGateway)
	require.NoError(t, err)
	require.Equal(t, uint64(2), gotIdx)

	// Deregister the single typical service and the service name should also be dropped
	idx++
	require.NoError(t, s.DeleteService(idx, "node1", "web", entMeta, ""))

	gotIdx, got, err = s.ServiceNamesOfKind(nil, structs.ServiceKindTypical)
	require.NoError(t, err)
	require.Equal(t, idx, gotIdx)
	require.Empty(t, got)

	// A ConnectEnabled entry should not be removed until all corresponding services are removed.
	{
		verifyConnectEnabled(connectEnabledIdx)
		// Add a connect-native service.
		idx++
		require.NoError(t, s.EnsureService(idx, "node1", &structs.NodeService{
			Kind:           structs.ServiceKindTypical,
			ID:             "foo",
			Service:        "foo",
			Address:        "5.5.5.5",
			Port:           5555,
			EnterpriseMeta: *entMeta,
			Connect: structs.ServiceConnect{
				Native: true,
			},
		}))
		verifyConnectEnabled(connectEnabledIdx)

		// Delete the proxy. This should not clean up the entry, because we still have a
		// connect-native service registered.
		idx++
		require.NoError(t, s.DeleteService(idx, "node1", "connect-proxy", entMeta, ""))
		verifyConnectEnabled(connectEnabledIdx)

		// Remove the connect-native service to clear out the connect-enabled entry.
		require.NoError(t, s.DeleteService(idx, "node1", "foo", entMeta, ""))
		gotIdx, gotNames, err := s.ServiceNamesOfKind(nil, structs.ServiceKindConnectEnabled)
		require.NoError(t, err)
		require.Equal(t, idx, gotIdx)
		require.Empty(t, gotNames)
	}
}

func assertMaxIndexes(t *testing.T, tx ReadTxn, expect map[string]uint64, skip ...string) {
	t.Helper()

	all := dumpMaxIndexes(t, tx)

	for _, index := range skip {
		if _, ok := all[index]; ok {
			delete(all, index)
		} else {
			t.Logf("index %q isn't even set; probably test assertion isn't relevant anymore", index)
		}
	}

	require.Equal(t, expect, all)

	// TODO
	// for _, index := range indexes {
	// 	require.Equal(t, expectIndex, maxIndexTxn(tx, index),
	// 		"index %s has the wrong value", index)
	// }
}

func dumpMaxIndexes(t *testing.T, tx ReadTxn) map[string]uint64 {
	out := make(map[string]uint64)

	iter, err := tx.Get(tableIndex, "id")
	require.NoError(t, err)

	for entry := iter.Next(); entry != nil; entry = iter.Next() {
		if idx, ok := entry.(*IndexEntry); ok {
			out[idx.Key] = idx.Value
		}
	}
	return out
}

func generateUUID() ([]byte, string) {
	buf := make([]byte, 16)
	if _, err := crand.Read(buf); err != nil {
		panic(fmt.Errorf("failed to read random bytes: %v", err))
	}
	uuid := fmt.Sprintf("%08x-%04x-%04x-%04x-%12x",
		buf[0:4],
		buf[4:6],
		buf[6:8],
		buf[8:10],
		buf[10:16])
	return buf, uuid
}

func setVirtualIPFlags(t *testing.T, s *Store) {
	require.NoError(t, s.SystemMetadataSet(0, &structs.SystemMetadataEntry{
		Key:   structs.SystemMetadataVirtualIPsEnabled,
		Value: "true",
	}))
	require.NoError(t, s.SystemMetadataSet(0, &structs.SystemMetadataEntry{
		Key:   structs.SystemMetadataTermGatewayVirtualIPsEnabled,
		Value: "true",
	}))
}

func assertDeepEqual(t *testing.T, x, y interface{}, opts ...cmp.Option) {
	t.Helper()
	if diff := cmp.Diff(x, y, opts...); diff != "" {
		t.Fatalf("assertion failed: values are not equal\n--- expected\n+++ actual\n%v", diff)
	}
}
