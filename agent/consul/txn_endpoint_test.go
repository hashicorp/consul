package consul

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	msgpackrpc "github.com/hashicorp/consul-net-rpc/net-rpc-msgpackrpc"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/types"
)

var testTxnRules = `
key "" {
	policy = "deny"
}
key "foo" {
	policy = "read"
}
key "test" {
	policy = "write"
}
key "test/priv" {
	policy = "read"
}

service "" {
	policy = "deny"
}
service "foo-svc" {
	policy = "read"
}
service "test-svc" {
	policy = "write"
}

node "" {
	policy = "deny"
}
node "foo-node" {
	policy = "read"
}
node "test-node" {
	policy = "write"
}
`

var testNodeID = "9749a7df-fac5-46b4-8078-32a3d96c59f3"

func TestTxn_CheckNotExists(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	apply := func(arg *structs.TxnRequest) (*structs.TxnResponse, error) {
		out := new(structs.TxnResponse)
		err := msgpackrpc.CallWithCodec(codec, "Txn.Apply", arg, out)
		return out, err
	}

	checkKeyNotExists := &structs.TxnRequest{
		Datacenter: "dc1",
		Ops: structs.TxnOps{
			{
				KV: &structs.TxnKVOp{
					Verb:   api.KVCheckNotExists,
					DirEnt: structs.DirEntry{Key: "test"},
				},
			},
		},
	}

	createKey := &structs.TxnRequest{
		Datacenter: "dc1",
		Ops: structs.TxnOps{
			{
				KV: &structs.TxnKVOp{
					Verb:   api.KVSet,
					DirEnt: structs.DirEntry{Key: "test"},
				},
			},
		},
	}

	if _, err := apply(checkKeyNotExists); err != nil {
		t.Fatalf("testing for non-existent key failed: %s", err)
	}
	if _, err := apply(createKey); err != nil {
		t.Fatalf("creating new key failed: %s", err)
	}
	out, err := apply(checkKeyNotExists)
	if err != nil || out == nil || len(out.Errors) != 1 || out.Errors[0].Error() != `op 0: key "test" exists` {
		t.Fatalf("testing for existent key failed: %#v", out)
	}
}

func TestTxn_Apply(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Do a super basic request. The state store test covers the details so
	// we just need to be sure that the transaction is sent correctly and
	// the results are converted appropriately.
	arg := structs.TxnRequest{
		Datacenter: "dc1",
		Ops: structs.TxnOps{
			&structs.TxnOp{
				KV: &structs.TxnKVOp{
					Verb: api.KVSet,
					DirEnt: structs.DirEntry{
						Key:   "test",
						Flags: 42,
						Value: []byte("test"),
					},
				},
			},
			&structs.TxnOp{
				KV: &structs.TxnKVOp{
					Verb: api.KVGet,
					DirEnt: structs.DirEntry{
						Key: "test",
					},
				},
			},
			&structs.TxnOp{
				Node: &structs.TxnNodeOp{
					Verb: api.NodeSet,
					Node: structs.Node{
						ID:      types.NodeID(testNodeID),
						Node:    "foo",
						Address: "127.0.0.1",
					},
				},
			},
			&structs.TxnOp{
				Node: &structs.TxnNodeOp{
					Verb: api.NodeGet,
					Node: structs.Node{
						ID:   types.NodeID(testNodeID),
						Node: "foo",
					},
				},
			},
			&structs.TxnOp{
				Service: &structs.TxnServiceOp{
					Verb: api.ServiceSet,
					Node: "foo",
					Service: structs.NodeService{
						ID:      "svc-foo",
						Service: "svc-foo",
						Address: "1.1.1.1",
					},
				},
			},
			&structs.TxnOp{
				Service: &structs.TxnServiceOp{
					Verb: api.ServiceGet,
					Node: "foo",
					Service: structs.NodeService{
						ID:      "svc-foo",
						Service: "svc-foo",
					},
				},
			},
			&structs.TxnOp{
				Check: &structs.TxnCheckOp{
					Verb: api.CheckSet,
					Check: structs.HealthCheck{
						Node:    "foo",
						CheckID: types.CheckID("check-foo"),
						Name:    "test",
						Status:  "passing",
					},
				},
			},
			&structs.TxnOp{
				Check: &structs.TxnCheckOp{
					Verb: api.CheckGet,
					Check: structs.HealthCheck{
						Node:    "foo",
						CheckID: types.CheckID("check-foo"),
						Name:    "test",
					},
				},
			},
		},
	}
	var out structs.TxnResponse
	if err := msgpackrpc.CallWithCodec(codec, "Txn.Apply", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(out.Errors) != 0 {
		t.Fatalf("errs: %v", out.Errors)
	}

	// Verify the state store directly.
	state := s1.fsm.State()
	_, d, err := state.KVSGet(nil, "test", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if d == nil {
		t.Fatalf("should not be nil")
	}
	if d.Flags != 42 ||
		!bytes.Equal(d.Value, []byte("test")) {
		t.Fatalf("bad: %v", d)
	}

	_, n, err := state.GetNode("foo", nil, "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if n.Node != "foo" || n.Address != "127.0.0.1" {
		t.Fatalf("bad: %v", err)
	}

	_, s, err := state.NodeService(nil, "foo", "svc-foo", nil, "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s.ID != "svc-foo" || s.Address != "1.1.1.1" {
		t.Fatalf("bad: %v", err)
	}

	_, c, err := state.NodeCheck("foo", types.CheckID("check-foo"), nil, "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if c.CheckID != "check-foo" || c.Status != "passing" || c.Name != "test" {
		t.Fatalf("bad: %v", err)
	}

	// Verify the transaction's return value.
	expected := structs.TxnResponse{
		Results: structs.TxnResults{
			&structs.TxnResult{
				KV: &structs.DirEntry{
					Key:   "test",
					Flags: 42,
					Value: nil,
					RaftIndex: structs.RaftIndex{
						CreateIndex: d.CreateIndex,
						ModifyIndex: d.ModifyIndex,
					},
					EnterpriseMeta: d.EnterpriseMeta,
				},
			},
			&structs.TxnResult{
				KV: &structs.DirEntry{
					Key:   "test",
					Flags: 42,
					Value: []byte("test"),
					RaftIndex: structs.RaftIndex{
						CreateIndex: d.CreateIndex,
						ModifyIndex: d.ModifyIndex,
					},
					EnterpriseMeta: d.EnterpriseMeta,
				},
			},
			&structs.TxnResult{
				Node: n,
			},
			&structs.TxnResult{
				Node: n,
			},
			&structs.TxnResult{
				Service: s,
			},
			&structs.TxnResult{
				Service: s,
			},
			&structs.TxnResult{
				Check: c,
			},
			&structs.TxnResult{
				Check: c,
			},
		},
	}
	require.Equal(t, expected, out)
}

func TestTxn_Apply_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Set up some state to read back.
	state := s1.fsm.State()
	d := &structs.DirEntry{
		Key:   "nope",
		Value: []byte("hello"),
	}
	require.NoError(t, state.KVSSet(1, d))

	node := &structs.Node{
		ID:   types.NodeID(testNodeID),
		Node: "nope",
	}
	require.NoError(t, state.EnsureNode(2, node))

	svc := structs.NodeService{ID: "nope", Service: "nope", Address: "127.0.0.1"}
	require.NoError(t, state.EnsureService(3, "nope", &svc))

	check := structs.HealthCheck{Node: "nope", CheckID: types.CheckID("nope")}
	state.EnsureCheck(4, &check)

	token := createTokenFull(t, rpcClient(t, s1), testTxnRules)
	id := token.SecretID

	// Set up a transaction where every operation should get blocked due to
	// ACLs.
	arg := structs.TxnRequest{
		Datacenter: "dc1",
		Ops: structs.TxnOps{
			&structs.TxnOp{
				KV: &structs.TxnKVOp{
					Verb: api.KVSet,
					DirEnt: structs.DirEntry{
						Key: "nope",
					},
				},
			},
			&structs.TxnOp{
				KV: &structs.TxnKVOp{
					Verb: api.KVDelete,
					DirEnt: structs.DirEntry{
						Key: "nope",
					},
				},
			},
			&structs.TxnOp{
				KV: &structs.TxnKVOp{
					Verb: api.KVDeleteCAS,
					DirEnt: structs.DirEntry{
						Key: "nope",
					},
				},
			},
			&structs.TxnOp{
				KV: &structs.TxnKVOp{
					Verb: api.KVDeleteTree,
					DirEnt: structs.DirEntry{
						Key: "nope",
					},
				},
			},
			&structs.TxnOp{
				KV: &structs.TxnKVOp{
					Verb: api.KVCAS,
					DirEnt: structs.DirEntry{
						Key: "nope",
					},
				},
			},
			&structs.TxnOp{
				KV: &structs.TxnKVOp{
					Verb: api.KVLock,
					DirEnt: structs.DirEntry{
						Key: "nope",
					},
				},
			},
			&structs.TxnOp{
				KV: &structs.TxnKVOp{
					Verb: api.KVUnlock,
					DirEnt: structs.DirEntry{
						Key: "nope",
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
					Verb: api.KVGetTree,
					DirEnt: structs.DirEntry{
						Key: "nope",
					},
				},
			},
			&structs.TxnOp{
				KV: &structs.TxnKVOp{
					Verb: api.KVCheckSession,
					DirEnt: structs.DirEntry{
						Key: "nope",
					},
				},
			},
			&structs.TxnOp{
				KV: &structs.TxnKVOp{
					Verb: api.KVCheckIndex,
					DirEnt: structs.DirEntry{
						Key: "nope",
					},
				},
			},
			&structs.TxnOp{
				KV: &structs.TxnKVOp{
					Verb: api.KVCheckNotExists,
					DirEnt: structs.DirEntry{
						Key: "nope",
					},
				},
			},
			&structs.TxnOp{
				Node: &structs.TxnNodeOp{
					Verb: api.NodeGet,
					Node: structs.Node{ID: node.ID, Node: node.Node},
				},
			},
			&structs.TxnOp{
				Node: &structs.TxnNodeOp{
					Verb: api.NodeSet,
					Node: structs.Node{ID: node.ID, Node: node.Node},
				},
			},
			&structs.TxnOp{
				Node: &structs.TxnNodeOp{
					Verb: api.NodeCAS,
					Node: structs.Node{ID: node.ID, Node: node.Node},
				},
			},
			&structs.TxnOp{
				Node: &structs.TxnNodeOp{
					Verb: api.NodeDelete,
					Node: structs.Node{ID: node.ID, Node: node.Node},
				},
			},
			&structs.TxnOp{
				Node: &structs.TxnNodeOp{
					Verb: api.NodeDeleteCAS,
					Node: structs.Node{ID: node.ID, Node: node.Node},
				},
			},
			&structs.TxnOp{
				Service: &structs.TxnServiceOp{
					Verb:    api.ServiceGet,
					Node:    "foo-node",
					Service: svc,
				},
			},
			&structs.TxnOp{
				Service: &structs.TxnServiceOp{
					Verb:    api.ServiceSet,
					Node:    "foo-node",
					Service: svc,
				},
			},
			&structs.TxnOp{
				Service: &structs.TxnServiceOp{
					Verb:    api.ServiceCAS,
					Node:    "foo-node",
					Service: svc,
				},
			},
			&structs.TxnOp{
				Service: &structs.TxnServiceOp{
					Verb:    api.ServiceDelete,
					Node:    "foo-node",
					Service: svc,
				},
			},
			&structs.TxnOp{
				Service: &structs.TxnServiceOp{
					Verb:    api.ServiceDeleteCAS,
					Node:    "foo-node",
					Service: svc,
				},
			},
			&structs.TxnOp{
				Check: &structs.TxnCheckOp{
					Verb:  api.CheckGet,
					Check: check,
				},
			},
			&structs.TxnOp{
				Check: &structs.TxnCheckOp{
					Verb:  api.CheckSet,
					Check: check,
				},
			},
			&structs.TxnOp{
				Check: &structs.TxnCheckOp{
					Verb:  api.CheckCAS,
					Check: check,
				},
			},
			&structs.TxnOp{
				Check: &structs.TxnCheckOp{
					Verb:  api.CheckDelete,
					Check: check,
				},
			},
			&structs.TxnOp{
				Check: &structs.TxnCheckOp{
					Verb:  api.CheckDeleteCAS,
					Check: check,
				},
			},
		},
		WriteRequest: structs.WriteRequest{
			Token: id,
		},
	}
	var out structs.TxnResponse
	if err := s1.RPC(context.Background(), "Txn.Apply", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify the transaction's return value.
	var outPos int
	for i, op := range arg.Ops {
		err := out.Errors[outPos]
		switch {
		case op.KV != nil:
			switch op.KV.Verb {
			case api.KVGet, api.KVGetTree:
				// These get filtered but won't result in an error.
			case api.KVSet, api.KVDelete, api.KVDeleteCAS, api.KVDeleteTree, api.KVCAS, api.KVLock, api.KVUnlock, api.KVCheckNotExists:
				require.Equal(t, err.OpIndex, i)
				acl.RequirePermissionDeniedMessage(t, err.What, token.AccessorID, nil, acl.ResourceKey, acl.AccessWrite, "nope")
				outPos++
			default:
				require.Equal(t, err.OpIndex, i)
				acl.RequirePermissionDeniedMessage(t, err.What, token.AccessorID, nil, acl.ResourceKey, acl.AccessRead, "nope")
				outPos++
			}
		case op.Node != nil:
			switch op.Node.Verb {
			case api.NodeGet:
				// These get filtered but won't result in an error.
			case api.NodeSet, api.NodeDelete, api.NodeDeleteCAS, api.NodeCAS:
				require.Equal(t, err.OpIndex, i)
				acl.RequirePermissionDeniedMessage(t, err.What, token.AccessorID, nil, acl.ResourceNode, acl.AccessWrite, "nope")
				outPos++
			default:
				require.Equal(t, err.OpIndex, i)
				acl.RequirePermissionDeniedMessage(t, err.What, token.AccessorID, nil, acl.ResourceNode, acl.AccessRead, "nope")
				outPos++
			}
		case op.Service != nil:
			switch op.Service.Verb {
			case api.ServiceGet:
				// These get filtered but won't result in an error.
			case api.ServiceSet, api.ServiceCAS, api.ServiceDelete, api.ServiceDeleteCAS:
				require.Equal(t, err.OpIndex, i)
				acl.RequirePermissionDeniedMessage(t, err.What, token.AccessorID, nil, acl.ResourceService, acl.AccessWrite, "nope")
				outPos++
			default:
				require.Equal(t, err.OpIndex, i)
				acl.RequirePermissionDeniedMessage(t, err.What, token.AccessorID, nil, acl.ResourceService, acl.AccessRead, "nope")
				outPos++
			}
		case op.Check != nil:
			switch op.Check.Verb {
			case api.CheckGet:
				// These get filtered but won't result in an error.
			case api.CheckSet, api.CheckCAS, api.CheckDelete, api.CheckDeleteCAS:
				require.Equal(t, err.OpIndex, i)
				acl.RequirePermissionDeniedMessage(t, err.What, token.AccessorID, nil, acl.ResourceNode, acl.AccessWrite, "nope")
				outPos++
			default:
				require.Equal(t, err.OpIndex, i)
				acl.RequirePermissionDeniedMessage(t, err.What, token.AccessorID, nil, acl.ResourceNode, acl.AccessRead, "nope")
				outPos++
			}
		}
	}
}

func TestTxn_Apply_LockDelay(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

	// Create and invalidate a session with a lock.
	state := s1.fsm.State()
	if err := state.EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	session := &structs.Session{
		ID:        generateUUID(),
		Node:      "foo",
		LockDelay: 50 * time.Millisecond,
	}
	if err := state.SessionCreate(2, session); err != nil {
		t.Fatalf("err: %v", err)
	}
	id := session.ID
	d := &structs.DirEntry{
		Key:     "test",
		Session: id,
	}
	if ok, err := state.KVSLock(3, d); err != nil || !ok {
		t.Fatalf("err: %v", err)
	}

	if err := state.SessionDestroy(4, id, nil); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make a new session that is valid.
	if err := state.SessionCreate(5, session); err != nil {
		t.Fatalf("err: %v", err)
	}
	validID := session.ID

	// Make a lock request via an atomic transaction.
	arg := structs.TxnRequest{
		Datacenter: "dc1",
		Ops: structs.TxnOps{
			&structs.TxnOp{
				KV: &structs.TxnKVOp{
					Verb: api.KVLock,
					DirEnt: structs.DirEntry{
						Key:     "test",
						Session: validID,
					},
				},
			},
		},
	}
	{
		var out structs.TxnResponse
		if err := msgpackrpc.CallWithCodec(codec, "Txn.Apply", &arg, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(out.Results) != 0 ||
			len(out.Errors) != 1 ||
			out.Errors[0].OpIndex != 0 ||
			!strings.Contains(out.Errors[0].What, "due to lock delay") {
			t.Fatalf("bad: %v", out)
		}
	}

	// Wait for lock-delay.
	time.Sleep(50 * time.Millisecond)

	// Should acquire.
	{
		var out structs.TxnResponse
		if err := msgpackrpc.CallWithCodec(codec, "Txn.Apply", &arg, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(out.Results) != 1 ||
			len(out.Errors) != 0 ||
			out.Results[0].KV.LockIndex != 2 {
			t.Fatalf("bad: %v", out)
		}
	}
}

func TestTxn_Read(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Put in a key to read back.
	state := s1.fsm.State()
	d := &structs.DirEntry{
		Key:   "test",
		Value: []byte("hello"),
	}
	if err := state.KVSSet(1, d); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Put in a node/check/service to read back.
	node := &structs.Node{
		ID:   types.NodeID(testNodeID),
		Node: "foo",
	}
	require.NoError(t, state.EnsureNode(2, node))

	svc := structs.NodeService{
		ID:             "svc-foo",
		Service:        "svc-foo",
		Address:        "127.0.0.1",
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	require.NoError(t, state.EnsureService(3, "foo", &svc))

	check := structs.HealthCheck{
		Node:           "foo",
		CheckID:        types.CheckID("check-foo"),
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	state.EnsureCheck(4, &check)

	// Do a super basic request. The state store test covers the details so
	// we just need to be sure that the transaction is sent correctly and
	// the results are converted appropriately.
	arg := structs.TxnReadRequest{
		Datacenter: "dc1",
		Ops: structs.TxnOps{
			&structs.TxnOp{
				KV: &structs.TxnKVOp{
					Verb: api.KVGet,
					DirEnt: structs.DirEntry{
						Key: "test",
					},
				},
			},
			&structs.TxnOp{
				Node: &structs.TxnNodeOp{
					Verb: api.NodeGet,
					Node: structs.Node{ID: node.ID, Node: node.Node},
				},
			},
			&structs.TxnOp{
				Service: &structs.TxnServiceOp{
					Verb:    api.ServiceGet,
					Node:    "foo",
					Service: svc,
				},
			},
			&structs.TxnOp{
				Check: &structs.TxnCheckOp{
					Verb:  api.CheckGet,
					Check: check,
				},
			},
		},
	}
	var out structs.TxnReadResponse
	if err := msgpackrpc.CallWithCodec(codec, "Txn.Read", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify the transaction's return value.
	svc.Weights = &structs.Weights{Passing: 1, Warning: 1}
	svc.RaftIndex = structs.RaftIndex{CreateIndex: 3, ModifyIndex: 3}

	entMeta := out.Results[0].KV.EnterpriseMeta
	expected := structs.TxnReadResponse{
		TxnResponse: structs.TxnResponse{
			Results: structs.TxnResults{
				&structs.TxnResult{
					KV: &structs.DirEntry{
						Key:   "test",
						Value: []byte("hello"),
						RaftIndex: structs.RaftIndex{
							CreateIndex: 1,
							ModifyIndex: 1,
						},
						EnterpriseMeta: entMeta,
					},
				},
				&structs.TxnResult{
					Node: node,
				},
				&structs.TxnResult{
					Service: &svc,
				},
				&structs.TxnResult{
					Check: &check,
				},
			},
		},
		QueryMeta: structs.QueryMeta{
			KnownLeader: true,
			Index:       1,
		},
	}
	require.Equal(t, expected, out)
}

func TestTxn_Read_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Put in a key to read back.
	state := s1.fsm.State()
	d := &structs.DirEntry{
		Key:   "nope",
		Value: []byte("hello"),
	}
	if err := state.KVSSet(1, d); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Put in a node/check/service to read back.
	node := &structs.Node{
		ID:   types.NodeID(testNodeID),
		Node: "nope",
	}
	require.NoError(t, state.EnsureNode(2, node))

	svc := structs.NodeService{ID: "nope", Service: "nope", Address: "127.0.0.1"}
	require.NoError(t, state.EnsureService(3, "nope", &svc))

	check := structs.HealthCheck{Node: "nope", CheckID: types.CheckID("nope")}
	state.EnsureCheck(4, &check)

	token := createTokenFull(t, codec, testTxnRules)

	t.Run("simple read operations (results get filtered out)", func(t *testing.T) {
		arg := structs.TxnReadRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Token: token.SecretID},
			Ops: structs.TxnOps{
				{
					KV: &structs.TxnKVOp{
						Verb: api.KVGet,
						DirEnt: structs.DirEntry{
							Key: "nope",
						},
					},
				},
				{
					KV: &structs.TxnKVOp{
						Verb: api.KVGetTree,
						DirEnt: structs.DirEntry{
							Key: "nope",
						},
					},
				},
			},
		}

		var out structs.TxnReadResponse
		err := msgpackrpc.CallWithCodec(codec, "Txn.Read", &arg, &out)
		require.NoError(t, err)
		require.Empty(t, out.Results)
		require.Empty(t, out.Errors)
		require.True(t, out.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})

	t.Run("complex operations (return permission denied errors)", func(t *testing.T) {
		arg := structs.TxnReadRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Token: token.SecretID},
			Ops: structs.TxnOps{
				{
					KV: &structs.TxnKVOp{
						Verb: api.KVCheckSession,
						DirEnt: structs.DirEntry{
							Key: "nope",
						},
					},
				},
				{
					KV: &structs.TxnKVOp{
						Verb: api.KVCheckIndex,
						DirEnt: structs.DirEntry{
							Key: "nope",
						},
					},
				},
			},
		}

		var out structs.TxnReadResponse
		err := msgpackrpc.CallWithCodec(codec, "Txn.Read", &arg, &out)
		require.NoError(t, err)
		acl.RequirePermissionDeniedMessage(t, out.Errors[0].What, token.AccessorID, nil, acl.ResourceKey, acl.AccessRead, "nope")
		acl.RequirePermissionDeniedMessage(t, out.Errors[1].What, token.AccessorID, nil, acl.ResourceKey, acl.AccessRead, "nope")

		require.Empty(t, out.Results)
	})
}
