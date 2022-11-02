package state

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/types"
)

//nolint:staticcheck
func TestStateStore_Txn_LegacyIntention(t *testing.T) {
	s := testStateStore(t)

	// Create some legacy intentions.
	ixn1 := &structs.Intention{
		ID:              testUUID(),
		SourceNS:        "default",
		SourceName:      "web",
		DestinationNS:   "default",
		DestinationName: "db",
		Meta:            map[string]string{},
	}
	ixn2 := &structs.Intention{
		ID:              testUUID(),
		SourceNS:        "default",
		SourceName:      "db",
		DestinationNS:   "default",
		DestinationName: "*",
		Action:          structs.IntentionActionDeny,
		Meta:            map[string]string{},
	}
	ixn3 := &structs.Intention{
		ID:              testUUID(),
		SourceNS:        "default",
		SourceName:      "foo",
		DestinationNS:   "default",
		DestinationName: "*",
		Meta:            map[string]string{},
	}

	// Write the first two to the state store, leave the third
	// to be created by the transaction operation.
	require.NoError(t, s.LegacyIntentionSet(1, ixn1))
	require.NoError(t, s.LegacyIntentionSet(2, ixn2))

	// Set up a transaction that hits every operation.
	ops := structs.TxnOps{
		&structs.TxnOp{
			Intention: &structs.TxnIntentionOp{
				Op:        structs.IntentionOpUpdate,
				Intention: ixn1,
			},
		},
		&structs.TxnOp{
			Intention: &structs.TxnIntentionOp{
				Op:        structs.IntentionOpDelete,
				Intention: ixn2,
			},
		},
		&structs.TxnOp{
			Intention: &structs.TxnIntentionOp{
				Op:        structs.IntentionOpCreate,
				Intention: ixn3,
			},
		},
	}
	results, errors := s.TxnRW(3, ops)
	if len(errors) > 0 {
		t.Fatalf("err: %v", errors)
	}

	// Make sure the response looks as expected.
	expected := structs.TxnResults{}
	require.Equal(t, expected, results)

	// Pull the resulting state store contents.
	idx, actual, fromConfig, err := s.Intentions(nil, nil)
	require.NoError(t, err)
	require.Equal(t, uint64(3), idx, "wrong index")
	require.False(t, fromConfig)

	// Make sure it looks as expected.
	intentions := structs.Intentions{
		&structs.Intention{
			ID:              ixn1.ID,
			SourceNS:        "default",
			SourceName:      "web",
			DestinationNS:   "default",
			DestinationName: "db",
			Meta:            map[string]string{},
			Precedence:      9,
			RaftIndex: structs.RaftIndex{
				CreateIndex: 1,
				ModifyIndex: 3,
			},
		},
		&structs.Intention{
			ID:              ixn3.ID,
			SourceNS:        "default",
			SourceName:      "foo",
			DestinationNS:   "default",
			DestinationName: "*",
			Meta:            map[string]string{},
			Precedence:      6,
			RaftIndex: structs.RaftIndex{
				CreateIndex: 3,
				ModifyIndex: 3,
			},
		},
	}
	require.Equal(t, intentions, actual)
}

func TestStateStore_Txn_Node(t *testing.T) {
	s := testStateStore(t)

	// Create some nodes.
	var nodes [5]structs.Node
	for i := 0; i < len(nodes); i++ {
		nodes[i] = structs.Node{
			Node: fmt.Sprintf("node%d", i+1),
			ID:   types.NodeID(testUUID()),
		}

		// Leave node5 to be created by an operation.
		if i < 5 {
			s.EnsureNode(uint64(i+1), &nodes[i])
		}
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
	}
	results, errors := s.TxnRW(8, ops)
	if len(errors) > 0 {
		t.Fatalf("err: %v", errors)
	}

	// Make sure the response looks as expected.
	nodes[1].Datacenter = "dc2"
	nodes[1].ModifyIndex = 8
	expected := structs.TxnResults{
		&structs.TxnResult{
			Node: &nodes[0],
		},
		&structs.TxnResult{
			Node: &nodes[4],
		},
		&structs.TxnResult{
			Node: &nodes[1],
		},
	}
	require.Equal(t, expected, results)

	// Pull the resulting state store contents.
	idx, actual, err := s.Nodes(nil, nil, "")
	require.NoError(t, err)
	if idx != 8 {
		t.Fatalf("bad index: %d", idx)
	}

	// Make sure it looks as expected.
	expectedNodes := structs.Nodes{&nodes[0], &nodes[1], &nodes[4]}
	require.Equal(t, expectedNodes, actual)
}

func TestStateStore_Txn_Service(t *testing.T) {
	s := testStateStore(t)

	testRegisterNode(t, s, 1, "node1")

	// Create some services.
	for i := 1; i <= 4; i++ {
		testRegisterService(t, s, uint64(i+1), "node1", fmt.Sprintf("svc%d", i))
	}

	// Set up a transaction that hits every operation.
	ops := structs.TxnOps{
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
	}
	results, errors := s.TxnRW(6, ops)
	if len(errors) > 0 {
		t.Fatalf("err: %v", errors)
	}

	// Make sure the response looks as expected.
	expected := structs.TxnResults{
		&structs.TxnResult{
			Service: &structs.NodeService{
				ID:      "svc1",
				Service: "svc1",
				Address: "1.1.1.1",
				Port:    1111,
				Weights: &structs.Weights{Passing: 1, Warning: 1},
				RaftIndex: structs.RaftIndex{
					CreateIndex: 2,
					ModifyIndex: 2,
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				Meta:           map[string]string{},
			},
		},
		&structs.TxnResult{
			Service: &structs.NodeService{
				ID:      "svc5",
				Weights: &structs.Weights{Passing: 1, Warning: 1},
				RaftIndex: structs.RaftIndex{
					CreateIndex: 6,
					ModifyIndex: 6,
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			},
		},
		&structs.TxnResult{
			Service: &structs.NodeService{
				ID:      "svc2",
				Tags:    []string{"modified"},
				Weights: &structs.Weights{Passing: 1, Warning: 1},
				RaftIndex: structs.RaftIndex{
					CreateIndex: 3,
					ModifyIndex: 6,
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			},
		},
	}
	require.Equal(t, expected, results)

	// Pull the resulting state store contents.
	idx, actual, err := s.NodeServices(nil, "node1", nil, "")
	require.NoError(t, err)
	if idx != 6 {
		t.Fatalf("bad index: %d", idx)
	}

	// Make sure it looks as expected.
	expectedServices := &structs.NodeServices{
		Node: &structs.Node{
			Node:      "node1",
			Partition: structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
			RaftIndex: structs.RaftIndex{
				CreateIndex: 1,
				ModifyIndex: 1,
			},
		},
		Services: map[string]*structs.NodeService{
			"svc1": {
				ID:      "svc1",
				Service: "svc1",
				Address: "1.1.1.1",
				Port:    1111,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 2,
					ModifyIndex: 2,
				},
				Weights:        &structs.Weights{Passing: 1, Warning: 1},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				Meta:           map[string]string{},
			},
			"svc5": {
				ID: "svc5",
				RaftIndex: structs.RaftIndex{
					CreateIndex: 6,
					ModifyIndex: 6,
				},
				Weights:        &structs.Weights{Passing: 1, Warning: 1},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			},
			"svc2": {
				ID:   "svc2",
				Tags: []string{"modified"},
				RaftIndex: structs.RaftIndex{
					CreateIndex: 3,
					ModifyIndex: 6,
				},
				Weights:        &structs.Weights{Passing: 1, Warning: 1},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			},
		},
	}
	require.Equal(t, expectedServices, actual)
}

func TestStateStore_Txn_Checks(t *testing.T) {
	s := testStateStore(t)

	testRegisterNode(t, s, 1, "node1")

	// Create some checks.
	for i := 1; i <= 4; i++ {
		testRegisterCheck(t, s, uint64(i+1), "node1", "", types.CheckID(fmt.Sprintf("check%d", i)), "failing")
	}

	// Set up a transaction that hits every operation.
	ops := structs.TxnOps{
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
	results, errors := s.TxnRW(6, ops)
	if len(errors) > 0 {
		t.Fatalf("err: %v", errors)
	}

	// Make sure the response looks as expected.
	expected := structs.TxnResults{
		&structs.TxnResult{
			Check: &structs.HealthCheck{
				Node:    "node1",
				CheckID: "check1",
				Status:  "failing",
				RaftIndex: structs.RaftIndex{
					CreateIndex: 2,
					ModifyIndex: 2,
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			},
		},
		&structs.TxnResult{
			Check: &structs.HealthCheck{
				Node:    "node1",
				CheckID: "check5",
				Status:  "passing",
				RaftIndex: structs.RaftIndex{
					CreateIndex: 6,
					ModifyIndex: 6,
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			},
		},
		&structs.TxnResult{
			Check: &structs.HealthCheck{
				Node:    "node1",
				CheckID: "check2",
				Status:  "warning",
				RaftIndex: structs.RaftIndex{
					CreateIndex: 3,
					ModifyIndex: 6,
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			},
		},
	}
	require.Equal(t, expected, results)

	// Pull the resulting state store contents.
	idx, actual, err := s.NodeChecks(nil, "node1", nil, "")
	require.NoError(t, err)
	if idx != 6 {
		t.Fatalf("bad index: %d", idx)
	}

	// Make sure it looks as expected.
	expectedChecks := structs.HealthChecks{
		&structs.HealthCheck{
			Node:    "node1",
			CheckID: "check1",
			Status:  "failing",
			RaftIndex: structs.RaftIndex{
				CreateIndex: 2,
				ModifyIndex: 2,
			},
			EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
		},
		&structs.HealthCheck{
			Node:    "node1",
			CheckID: "check2",
			Status:  "warning",
			RaftIndex: structs.RaftIndex{
				CreateIndex: 3,
				ModifyIndex: 6,
			},
			EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
		},
		&structs.HealthCheck{
			Node:    "node1",
			CheckID: "check5",
			Status:  "passing",
			RaftIndex: structs.RaftIndex{
				CreateIndex: 6,
				ModifyIndex: 6,
			},
			EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
		},
	}
	require.Equal(t, expectedChecks, actual)
}

func TestStateStore_Txn_KVS(t *testing.T) {
	s := testStateStore(t)

	// Create KV entries in the state store.
	testSetKey(t, s, 1, "foo/delete", "bar", nil)
	testSetKey(t, s, 2, "foo/bar/baz", "baz", nil)
	testSetKey(t, s, 3, "foo/bar/zip", "zip", nil)
	testSetKey(t, s, 4, "foo/zorp", "zorp", nil)
	testSetKey(t, s, 5, "foo/update", "stale", nil)

	// Make a real session.
	testRegisterNode(t, s, 6, "node1")
	session := testUUID()
	if err := s.SessionCreate(7, &structs.Session{ID: session, Node: "node1"}); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Set up a transaction that hits every operation.
	ops := structs.TxnOps{
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: api.KVGetTree,
				DirEnt: structs.DirEntry{
					Key: "foo/bar",
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: api.KVSet,
				DirEnt: structs.DirEntry{
					Key:   "foo/new",
					Value: []byte("one"),
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: api.KVDelete,
				DirEnt: structs.DirEntry{
					Key: "foo/zorp",
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: api.KVDeleteCAS,
				DirEnt: structs.DirEntry{
					Key: "foo/delete",
					RaftIndex: structs.RaftIndex{
						ModifyIndex: 1,
					},
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: api.KVDeleteTree,
				DirEnt: structs.DirEntry{
					Key: "foo/bar",
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: api.KVGet,
				DirEnt: structs.DirEntry{
					Key: "foo/update",
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: api.KVGetOrEmpty,
				DirEnt: structs.DirEntry{
					Key: "foo/update",
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: api.KVGetOrEmpty,
				DirEnt: structs.DirEntry{
					Key: "foo/not-exists",
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: api.KVCheckIndex,
				DirEnt: structs.DirEntry{
					Key: "foo/update",
					RaftIndex: structs.RaftIndex{
						ModifyIndex: 5,
					},
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: api.KVCAS,
				DirEnt: structs.DirEntry{
					Key:   "foo/update",
					Value: []byte("new"),
					RaftIndex: structs.RaftIndex{
						ModifyIndex: 5,
					},
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: api.KVGet,
				DirEnt: structs.DirEntry{
					Key: "foo/update",
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: api.KVCheckIndex,
				DirEnt: structs.DirEntry{
					Key: "foo/update",
					RaftIndex: structs.RaftIndex{
						ModifyIndex: 8,
					},
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: api.KVLock,
				DirEnt: structs.DirEntry{
					Key:     "foo/lock",
					Session: session,
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: api.KVCheckSession,
				DirEnt: structs.DirEntry{
					Key:     "foo/lock",
					Session: session,
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: api.KVUnlock,
				DirEnt: structs.DirEntry{
					Key:     "foo/lock",
					Session: session,
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: api.KVCheckSession,
				DirEnt: structs.DirEntry{
					Key:     "foo/lock",
					Session: "",
				},
			},
		},
	}
	results, errors := s.TxnRW(8, ops)
	if len(errors) > 0 {
		t.Fatalf("err: %v", errors)
	}

	// Make sure the response looks as expected.
	expected := structs.TxnResults{
		&structs.TxnResult{
			KV: &structs.DirEntry{
				Key:   "foo/bar/baz",
				Value: []byte("baz"),
				RaftIndex: structs.RaftIndex{
					CreateIndex: 2,
					ModifyIndex: 2,
				},
			},
		},
		&structs.TxnResult{
			KV: &structs.DirEntry{
				Key:   "foo/bar/zip",
				Value: []byte("zip"),
				RaftIndex: structs.RaftIndex{
					CreateIndex: 3,
					ModifyIndex: 3,
				},
			},
		},
		&structs.TxnResult{
			KV: &structs.DirEntry{
				Key: "foo/new",
				RaftIndex: structs.RaftIndex{
					CreateIndex: 8,
					ModifyIndex: 8,
				},
			},
		},
		&structs.TxnResult{
			KV: &structs.DirEntry{
				Key:   "foo/update",
				Value: []byte("stale"),
				RaftIndex: structs.RaftIndex{
					CreateIndex: 5,
					ModifyIndex: 5,
				},
			},
		},
		&structs.TxnResult{
			KV: &structs.DirEntry{
				Key:   "foo/update",
				Value: []byte("stale"),
				RaftIndex: structs.RaftIndex{
					CreateIndex: 5,
					ModifyIndex: 5,
				},
			},
		},
		&structs.TxnResult{
			KV: &structs.DirEntry{
				Key:   "foo/not-exists",
				Value: nil,
			},
		},
		&structs.TxnResult{
			KV: &structs.DirEntry{

				Key: "foo/update",
				RaftIndex: structs.RaftIndex{
					CreateIndex: 5,
					ModifyIndex: 5,
				},
			},
		},
		&structs.TxnResult{
			KV: &structs.DirEntry{
				Key: "foo/update",
				RaftIndex: structs.RaftIndex{
					CreateIndex: 5,
					ModifyIndex: 8,
				},
			},
		},
		&structs.TxnResult{
			KV: &structs.DirEntry{
				Key:   "foo/update",
				Value: []byte("new"),
				RaftIndex: structs.RaftIndex{
					CreateIndex: 5,
					ModifyIndex: 8,
				},
			},
		},
		&structs.TxnResult{
			KV: &structs.DirEntry{
				Key: "foo/update",
				RaftIndex: structs.RaftIndex{
					CreateIndex: 5,
					ModifyIndex: 8,
				},
			},
		},
		&structs.TxnResult{
			KV: &structs.DirEntry{
				Key:       "foo/lock",
				Session:   session,
				LockIndex: 1,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 8,
					ModifyIndex: 8,
				},
			},
		},
		&structs.TxnResult{
			KV: &structs.DirEntry{
				Key:       "foo/lock",
				Session:   session,
				LockIndex: 1,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 8,
					ModifyIndex: 8,
				},
			},
		},
		&structs.TxnResult{
			KV: &structs.DirEntry{
				Key:       "foo/lock",
				LockIndex: 1,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 8,
					ModifyIndex: 8,
				},
			},
		},
		&structs.TxnResult{
			KV: &structs.DirEntry{
				Key:       "foo/lock",
				LockIndex: 1,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 8,
					ModifyIndex: 8,
				},
			},
		},
	}
	if len(results) != len(expected) {
		t.Fatalf("bad: %v", results)
	}
	for i, e := range expected {
		if e.KV.Key != results[i].KV.Key {
			t.Fatalf("expected key %s, got %s", e.KV.Key, results[i].KV.Key)
		}
		if e.KV.LockIndex != results[i].KV.LockIndex {
			t.Fatalf("expected lock index %d, got %d", e.KV.LockIndex, results[i].KV.LockIndex)
		}
		if e.KV.CreateIndex != results[i].KV.CreateIndex {
			t.Fatalf("expected create index %d, got %d", e.KV.CreateIndex, results[i].KV.CreateIndex)
		}
		if e.KV.ModifyIndex != results[i].KV.ModifyIndex {
			t.Fatalf("expected modify index %d, got %d", e.KV.ModifyIndex, results[i].KV.ModifyIndex)
		}
	}

	// Pull the resulting state store contents.
	idx, actual, err := s.KVSList(nil, "", nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 8 {
		t.Fatalf("bad index: %d", idx)
	}

	// Make sure it looks as expected.
	entries := structs.DirEntries{
		&structs.DirEntry{
			Key:       "foo/lock",
			LockIndex: 1,
			RaftIndex: structs.RaftIndex{
				CreateIndex: 8,
				ModifyIndex: 8,
			},
		},
		&structs.DirEntry{
			Key:   "foo/new",
			Value: []byte("one"),
			RaftIndex: structs.RaftIndex{
				CreateIndex: 8,
				ModifyIndex: 8,
			},
		},
		&structs.DirEntry{
			Key:   "foo/update",
			Value: []byte("new"),
			RaftIndex: structs.RaftIndex{
				CreateIndex: 5,
				ModifyIndex: 8,
			},
		},
	}
	if len(actual) != len(entries) {
		t.Fatalf("bad len: %d != %d", len(actual), len(entries))
	}
	for i, e := range entries {
		if e.Key != actual[i].Key {
			t.Fatalf("expected key %s, got %s", e.Key, actual[i].Key)
		}
		if string(e.Value) != string(actual[i].Value) {
			t.Fatalf("expected value %s, got %s", e.Value, actual[i].Value)
		}
		if e.LockIndex != actual[i].LockIndex {
			t.Fatalf("expected lock index %d, got %d", e.LockIndex, actual[i].LockIndex)
		}
		if e.CreateIndex != actual[i].CreateIndex {
			t.Fatalf("expected create index %d, got %d", e.CreateIndex, actual[i].CreateIndex)
		}
		if e.ModifyIndex != actual[i].ModifyIndex {
			t.Fatalf("expected modify index %d, got %d", e.ModifyIndex, actual[i].ModifyIndex)
		}
	}
}

func TestStateStore_Txn_KVS_Rollback(t *testing.T) {
	s := testStateStore(t)

	// Create KV entries in the state store.
	testSetKey(t, s, 1, "foo/delete", "bar", nil)
	testSetKey(t, s, 2, "foo/update", "stale", nil)

	testRegisterNode(t, s, 3, "node1")
	session := testUUID()
	if err := s.SessionCreate(4, &structs.Session{ID: session, Node: "node1"}); err != nil {
		t.Fatalf("err: %s", err)
	}
	ok, err := s.KVSLock(5, &structs.DirEntry{Key: "foo/lock", Value: []byte("foo"), Session: session})
	if !ok || err != nil {
		t.Fatalf("didn't get the lock: %v %s", ok, err)
	}

	bogus := testUUID()
	if err := s.SessionCreate(6, &structs.Session{ID: bogus, Node: "node1"}); err != nil {
		t.Fatalf("err: %s", err)
	}

	// This function verifies that the state store wasn't changed.
	verifyStateStore := func(desc string) {
		idx, actual, err := s.KVSList(nil, "", nil)
		if err != nil {
			t.Fatalf("err (%s): %s", desc, err)
		}
		if idx != 5 {
			t.Fatalf("bad index (%s): %d", desc, idx)
		}

		// Make sure it looks as expected.
		entries := structs.DirEntries{
			&structs.DirEntry{
				Key:   "foo/delete",
				Value: []byte("bar"),
				RaftIndex: structs.RaftIndex{
					CreateIndex: 1,
					ModifyIndex: 1,
				},
			},
			&structs.DirEntry{
				Key:       "foo/lock",
				Value:     []byte("foo"),
				LockIndex: 1,
				Session:   session,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 5,
					ModifyIndex: 5,
				},
			},
			&structs.DirEntry{
				Key:   "foo/update",
				Value: []byte("stale"),
				RaftIndex: structs.RaftIndex{
					CreateIndex: 2,
					ModifyIndex: 2,
				},
			},
		}
		if len(actual) != len(entries) {
			t.Fatalf("bad len (%s): %d != %d", desc, len(actual), len(entries))
		}
		for i, e := range entries {
			if e.Key != actual[i].Key {
				t.Fatalf("expected key %s, got %s", e.Key, actual[i].Key)
			}
			if string(e.Value) != string(actual[i].Value) {
				t.Fatalf("expected value %s, got %s", e.Value, actual[i].Value)
			}
			if e.LockIndex != actual[i].LockIndex {
				t.Fatalf("expected lock index %d, got %d", e.LockIndex, actual[i].LockIndex)
			}
			if e.CreateIndex != actual[i].CreateIndex {
				t.Fatalf("expected create index %d, got %d", e.CreateIndex, actual[i].CreateIndex)
			}
			if e.ModifyIndex != actual[i].ModifyIndex {
				t.Fatalf("expected modify index %d, got %d", e.ModifyIndex, actual[i].ModifyIndex)
			}
		}
	}
	verifyStateStore("initial")

	// Set up a transaction that fails every operation.
	ops := structs.TxnOps{
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: api.KVCAS,
				DirEnt: structs.DirEntry{
					Key:   "foo/update",
					Value: []byte("new"),
					RaftIndex: structs.RaftIndex{
						ModifyIndex: 1,
					},
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: api.KVLock,
				DirEnt: structs.DirEntry{
					Key:     "foo/lock",
					Session: bogus,
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: api.KVUnlock,
				DirEnt: structs.DirEntry{
					Key:     "foo/lock",
					Session: bogus,
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: api.KVCheckSession,
				DirEnt: structs.DirEntry{
					Key:     "foo/lock",
					Session: bogus,
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: api.KVGet,
				DirEnt: structs.DirEntry{
					Key: "nope",
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: api.KVCheckSession,
				DirEnt: structs.DirEntry{
					Key:     "nope",
					Session: bogus,
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: api.KVCheckIndex,
				DirEnt: structs.DirEntry{
					Key: "foo/lock",
					RaftIndex: structs.RaftIndex{
						ModifyIndex: 6,
					},
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: api.KVCheckIndex,
				DirEnt: structs.DirEntry{
					Key: "nope",
					RaftIndex: structs.RaftIndex{
						ModifyIndex: 6,
					},
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: "nope",
				DirEnt: structs.DirEntry{
					Key: "foo/delete",
				},
			},
		},
	}
	results, errors := s.TxnRW(7, ops)
	if len(errors) != len(ops) {
		t.Fatalf("bad len: %d != %d", len(errors), len(ops))
	}
	if len(results) != 0 {
		t.Fatalf("bad len: %d != 0", len(results))
	}
	verifyStateStore("after")

	// Make sure the errors look reasonable.
	expected := []string{
		"index is stale",
		"lock is already held",
		"lock isn't held, or is held by another session",
		"current session",
		`key "nope" doesn't exist`,
		`key "nope" doesn't exist`,
		"current modify index",
		`key "nope" doesn't exist`,
		"unknown KV verb",
	}
	if len(errors) != len(expected) {
		t.Fatalf("bad len: %d != %d", len(errors), len(expected))
	}
	for i, msg := range expected {
		if errors[i].OpIndex != i {
			t.Fatalf("bad index: %d != %d", i, errors[i].OpIndex)
		}
		if !strings.Contains(errors[i].Error(), msg) {
			t.Fatalf("bad %d: %v", i, errors[i].Error())
		}
	}
}

func TestStateStore_Txn_KVS_RO(t *testing.T) {
	s := testStateStore(t)

	// Create KV entries in the state store.
	testSetKey(t, s, 1, "foo", "bar", nil)
	testSetKey(t, s, 2, "foo/bar/baz", "baz", nil)
	testSetKey(t, s, 3, "foo/bar/zip", "zip", nil)

	// Set up a transaction that hits all the read-only operations.
	ops := structs.TxnOps{
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: api.KVGetTree,
				DirEnt: structs.DirEntry{
					Key: "foo/bar",
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: api.KVGet,
				DirEnt: structs.DirEntry{
					Key: "foo",
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: api.KVCheckSession,
				DirEnt: structs.DirEntry{
					Key:     "foo/bar/baz",
					Session: "",
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: api.KVCheckSession,
				DirEnt: structs.DirEntry{
					Key: "foo/bar/zip",
					RaftIndex: structs.RaftIndex{
						ModifyIndex: 3,
					},
				},
			},
		},
	}
	results, errors := s.TxnRO(ops)
	if len(errors) > 0 {
		t.Fatalf("err: %v", errors)
	}

	// Make sure the response looks as expected.
	expected := structs.TxnResults{
		&structs.TxnResult{
			KV: &structs.DirEntry{
				Key:   "foo/bar/baz",
				Value: []byte("baz"),
				RaftIndex: structs.RaftIndex{
					CreateIndex: 2,
					ModifyIndex: 2,
				},
			},
		},
		&structs.TxnResult{
			KV: &structs.DirEntry{
				Key:   "foo/bar/zip",
				Value: []byte("zip"),
				RaftIndex: structs.RaftIndex{
					CreateIndex: 3,
					ModifyIndex: 3,
				},
			},
		},
		&structs.TxnResult{
			KV: &structs.DirEntry{
				Key:   "foo",
				Value: []byte("bar"),
				RaftIndex: structs.RaftIndex{
					CreateIndex: 1,
					ModifyIndex: 1,
				},
			},
		},
		&structs.TxnResult{
			KV: &structs.DirEntry{
				Key: "foo/bar/baz",
				RaftIndex: structs.RaftIndex{
					CreateIndex: 2,
					ModifyIndex: 2,
				},
			},
		},
		&structs.TxnResult{
			KV: &structs.DirEntry{
				Key: "foo/bar/zip",
				RaftIndex: structs.RaftIndex{
					CreateIndex: 3,
					ModifyIndex: 3,
				},
			},
		},
	}
	if len(results) != len(expected) {
		t.Fatalf("bad: %v", results)
	}
	for i, e := range expected {
		if e.KV.Key != results[i].KV.Key {
			t.Fatalf("expected key %s, got %s", e.KV.Key, results[i].KV.Key)
		}
		if e.KV.LockIndex != results[i].KV.LockIndex {
			t.Fatalf("expected lock index %d, got %d", e.KV.LockIndex, results[i].KV.LockIndex)
		}
		if e.KV.CreateIndex != results[i].KV.CreateIndex {
			t.Fatalf("expected create index %d, got %d", e.KV.CreateIndex, results[i].KV.CreateIndex)
		}
		if e.KV.ModifyIndex != results[i].KV.ModifyIndex {
			t.Fatalf("expected modify index %d, got %d", e.KV.ModifyIndex, results[i].KV.ModifyIndex)
		}
	}
}

func TestStateStore_Txn_KVS_RO_Safety(t *testing.T) {
	s := testStateStore(t)

	// Create KV entries in the state store.
	testSetKey(t, s, 1, "foo", "bar", nil)
	testSetKey(t, s, 2, "foo/bar/baz", "baz", nil)
	testSetKey(t, s, 3, "foo/bar/zip", "zip", nil)

	// Set up a transaction that hits all the read-only operations.
	ops := structs.TxnOps{
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: api.KVSet,
				DirEnt: structs.DirEntry{
					Key:   "foo",
					Value: []byte("nope"),
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: api.KVDelete,
				DirEnt: structs.DirEntry{
					Key: "foo/bar/baz",
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: api.KVDeleteTree,
				DirEnt: structs.DirEntry{
					Key: "foo/bar",
				},
			},
		},
	}
	results, errors := s.TxnRO(ops)
	if len(results) > 0 {
		t.Fatalf("bad: %v", results)
	}
	if len(errors) != len(ops) {
		t.Fatalf("bad len: %d != %d", len(errors), len(ops))
	}

	// Make sure the errors look reasonable (tombstone inserts cause the
	// insert errors during the delete operations).
	expected := []string{
		"cannot insert in read-only transaction",
		"cannot insert in read-only transaction",
		"failed recursive deleting kvs entry",
	}
	if len(errors) != len(expected) {
		t.Fatalf("bad len: %d != %d", len(errors), len(expected))
	}
	for i, msg := range expected {
		if errors[i].OpIndex != i {
			t.Fatalf("bad index: %d != %d", i, errors[i].OpIndex)
		}
		if !strings.Contains(errors[i].Error(), msg) {
			t.Fatalf("bad %d: %v", i, errors[i].Error())
		}
	}
}

func TestStateStore_Txn_KVS_ModifyIndexes(t *testing.T) {
	s := testStateStore(t)

	// Create KV entries in the state store.
	testSetKey(t, s, 1, "foo/a", "bar", nil)
	testSetKey(t, s, 2, "foo/b", "bar", nil)

	// Set up a transaction that actually changes `a`,
	// but passes original value for `b`.
	ops := structs.TxnOps{
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: api.KVCAS,
				DirEnt: structs.DirEntry{
					Key:   "foo/a",
					Value: []byte("new"),
					RaftIndex: structs.RaftIndex{
						ModifyIndex: 1,
					},
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: api.KVCAS,
				DirEnt: structs.DirEntry{
					Key:   "foo/b",
					Value: []byte("bar"),
					RaftIndex: structs.RaftIndex{
						ModifyIndex: 2,
					},
				},
			},
		},
	}
	results, errors := s.TxnRW(3, ops)
	if len(errors) > 0 {
		t.Fatalf("err: %v", errors)
	}

	// Make sure the response looks as expected.
	expected := structs.TxnResults{
		&structs.TxnResult{
			KV: &structs.DirEntry{
				Key: "foo/a",
				RaftIndex: structs.RaftIndex{
					CreateIndex: 1,
					ModifyIndex: 3,
				},
			},
		},
		&structs.TxnResult{
			KV: &structs.DirEntry{
				Key: "foo/b",
				RaftIndex: structs.RaftIndex{
					CreateIndex: 2,
					ModifyIndex: 2,
				},
			},
		},
	}
	if len(results) != len(expected) {
		t.Fatalf("bad: %v", results)
	}
	for i, e := range expected {
		if e.KV.Key != results[i].KV.Key {
			t.Fatalf("expected key %s, got %s", e.KV.Key, results[i].KV.Key)
		}
		if e.KV.LockIndex != results[i].KV.LockIndex {
			t.Fatalf("expected lock index %d, got %d", e.KV.LockIndex, results[i].KV.LockIndex)
		}
		if e.KV.CreateIndex != results[i].KV.CreateIndex {
			t.Fatalf("expected create index %d, got %d", e.KV.CreateIndex, results[i].KV.CreateIndex)
		}
		if e.KV.ModifyIndex != results[i].KV.ModifyIndex {
			t.Fatalf("expected modify index %d, got %d", e.KV.ModifyIndex, results[i].KV.ModifyIndex)
		}
	}

	// Pull the resulting state store contents.
	idx, actual, err := s.KVSList(nil, "", nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 3 {
		t.Fatalf("bad index: %d", idx)
	}

	// Make sure it looks as expected.
	entries := structs.DirEntries{
		&structs.DirEntry{
			Key:   "foo/a",
			Value: []byte("new"),
			RaftIndex: structs.RaftIndex{
				CreateIndex: 1,
				ModifyIndex: 3,
			},
		},
		&structs.DirEntry{
			Key:   "foo/b",
			Value: []byte("bar"),
			RaftIndex: structs.RaftIndex{
				CreateIndex: 2,
				ModifyIndex: 2,
			},
		},
	}
	if len(actual) != len(entries) {
		t.Fatalf("bad len: %d != %d", len(actual), len(entries))
	}
	for i, e := range entries {
		if e.Key != actual[i].Key {
			t.Fatalf("expected key %s, got %s", e.Key, actual[i].Key)
		}
		if string(e.Value) != string(actual[i].Value) {
			t.Fatalf("expected value %s, got %s", e.Value, actual[i].Value)
		}
		if e.LockIndex != actual[i].LockIndex {
			t.Fatalf("expected lock index %d, got %d", e.LockIndex, actual[i].LockIndex)
		}
		if e.CreateIndex != actual[i].CreateIndex {
			t.Fatalf("expected create index %d, got %d", e.CreateIndex, actual[i].CreateIndex)
		}
		if e.ModifyIndex != actual[i].ModifyIndex {
			t.Fatalf("expected modify index %d, got %d", e.ModifyIndex, actual[i].ModifyIndex)
		}
	}
}
