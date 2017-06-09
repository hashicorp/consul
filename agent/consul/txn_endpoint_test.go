package consul

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/consul/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/net-rpc-msgpackrpc"
)

func TestTxn_CheckNotExists(t *testing.T) {
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
		},
	}
	var out structs.TxnResponse
	if err := msgpackrpc.CallWithCodec(codec, "Txn.Apply", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify the state store directly.
	state := s1.fsm.State()
	_, d, err := state.KVSGet(nil, "test")
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
				},
			},
		},
	}
	if !reflect.DeepEqual(out, expected) {
		t.Fatalf("bad %v", out)
	}
}

func TestTxn_Apply_ACLDeny(t *testing.T) {
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
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

	// Create the ACL.
	var id string
	{
		arg := structs.ACLRequest{
			Datacenter: "dc1",
			Op:         structs.ACLSet,
			ACL: structs.ACL{
				Name:  "User token",
				Type:  structs.ACLTypeClient,
				Rules: testListRules,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		if err := msgpackrpc.CallWithCodec(codec, "ACL.Apply", &arg, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

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
		},
		WriteRequest: structs.WriteRequest{
			Token: id,
		},
	}
	var out structs.TxnResponse
	if err := msgpackrpc.CallWithCodec(codec, "Txn.Apply", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify the transaction's return value.
	var expected structs.TxnResponse
	for i, op := range arg.Ops {
		switch op.KV.Verb {
		case api.KVGet, api.KVGetTree:
			// These get filtered but won't result in an error.

		default:
			expected.Errors = append(expected.Errors, &structs.TxnError{
				OpIndex: i,
				What:    errPermissionDenied.Error(),
			})
		}
	}
	if !reflect.DeepEqual(out, expected) {
		t.Fatalf("bad %v", out)
	}
}

func TestTxn_Apply_LockDelay(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

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
	if err := state.SessionDestroy(4, id); err != nil {
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
		},
	}
	var out structs.TxnReadResponse
	if err := msgpackrpc.CallWithCodec(codec, "Txn.Read", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify the transaction's return value.
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
					},
				},
			},
		},
		QueryMeta: structs.QueryMeta{
			KnownLeader: true,
		},
	}
	if !reflect.DeepEqual(out, expected) {
		t.Fatalf("bad %v", out)
	}
}

func TestTxn_Read_ACLDeny(t *testing.T) {
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
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

	// Create the ACL.
	var id string
	{
		arg := structs.ACLRequest{
			Datacenter: "dc1",
			Op:         structs.ACLSet,
			ACL: structs.ACL{
				Name:  "User token",
				Type:  structs.ACLTypeClient,
				Rules: testListRules,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		if err := msgpackrpc.CallWithCodec(codec, "ACL.Apply", &arg, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Set up a transaction where every operation should get blocked due to
	// ACLs.
	arg := structs.TxnReadRequest{
		Datacenter: "dc1",
		Ops: structs.TxnOps{
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
		},
		QueryOptions: structs.QueryOptions{
			Token: id,
		},
	}
	var out structs.TxnReadResponse
	if err := msgpackrpc.CallWithCodec(codec, "Txn.Read", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify the transaction's return value.
	expected := structs.TxnReadResponse{
		QueryMeta: structs.QueryMeta{
			KnownLeader: true,
		},
	}
	for i, op := range arg.Ops {
		switch op.KV.Verb {
		case api.KVGet, api.KVGetTree:
			// These get filtered but won't result in an error.

		default:
			expected.Errors = append(expected.Errors, &structs.TxnError{
				OpIndex: i,
				What:    errPermissionDenied.Error(),
			})
		}
	}
	if !reflect.DeepEqual(out, expected) {
		t.Fatalf("bad %v", out)
	}
}
