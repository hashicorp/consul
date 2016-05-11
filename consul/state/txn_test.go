package state

import (
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/consul/consul/structs"
)

func TestStateStore_Txn_KVS(t *testing.T) {
	s := testStateStore(t)

	// Create kvs results in the state store.
	testSetKey(t, s, 1, "foo/delete", "bar")
	testSetKey(t, s, 2, "foo/bar/baz", "baz")
	testSetKey(t, s, 3, "foo/bar/zip", "zip")
	testSetKey(t, s, 4, "foo/zorp", "zorp")
	testSetKey(t, s, 5, "foo/update", "stale")

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
				Verb: structs.KVSSet,
				DirEnt: structs.DirEntry{
					Key:   "foo/new",
					Value: []byte("one"),
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: structs.KVSDelete,
				DirEnt: structs.DirEntry{
					Key: "foo/zorp",
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: structs.KVSDeleteCAS,
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
				Verb: structs.KVSDeleteTree,
				DirEnt: structs.DirEntry{
					Key: "foo/bar",
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: structs.KVSGet,
				DirEnt: structs.DirEntry{
					Key: "foo/update",
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: structs.KVSCheckIndex,
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
				Verb: structs.KVSCAS,
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
				Verb: structs.KVSGet,
				DirEnt: structs.DirEntry{
					Key: "foo/update",
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: structs.KVSGet,
				DirEnt: structs.DirEntry{
					Key: "not/there",
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: structs.KVSCheckIndex,
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
				Verb: structs.KVSGet,
				DirEnt: structs.DirEntry{
					Key: "foo/lock",
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: structs.KVSLock,
				DirEnt: structs.DirEntry{
					Key:     "foo/lock",
					Session: session,
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: structs.KVSCheckSession,
				DirEnt: structs.DirEntry{
					Key:     "foo/lock",
					Session: session,
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: structs.KVSUnlock,
				DirEnt: structs.DirEntry{
					Key:     "foo/lock",
					Session: session,
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: structs.KVSCheckSession,
				DirEnt: structs.DirEntry{
					Key:     "foo/lock",
					Session: "",
				},
			},
		},
	}
	results, errors := s.TxnRun(8, ops)
	if len(errors) > 0 {
		t.Fatalf("err: %v", errors)
	}
	if len(results) != len(ops) {
		t.Fatalf("bad len: %d != %d", len(results), len(ops))
	}

	// Make sure the response looks as expected.
	expected := structs.TxnResults{
		&structs.TxnResult{
			KV: &structs.TxnKVResult{
				DirEnt: &structs.DirEntry{
					Key: "foo/new",
					RaftIndex: structs.RaftIndex{
						CreateIndex: 8,
						ModifyIndex: 8,
					},
				},
			},
		},
		&structs.TxnResult{}, // delete
		&structs.TxnResult{}, // delete tree
		&structs.TxnResult{}, // delete CAS
		&structs.TxnResult{
			KV: &structs.TxnKVResult{
				DirEnt: &structs.DirEntry{
					Key:   "foo/update",
					Value: []byte("stale"),
					RaftIndex: structs.RaftIndex{
						CreateIndex: 5,
						ModifyIndex: 5,
					},
				},
			},
		},
		&structs.TxnResult{
			KV: &structs.TxnKVResult{
				DirEnt: &structs.DirEntry{

					Key: "foo/update",
					RaftIndex: structs.RaftIndex{
						CreateIndex: 5,
						ModifyIndex: 5,
					},
				},
			},
		},
		&structs.TxnResult{
			KV: &structs.TxnKVResult{
				DirEnt: &structs.DirEntry{
					Key: "foo/update",
					RaftIndex: structs.RaftIndex{
						CreateIndex: 5,
						ModifyIndex: 8,
					},
				},
			},
		},
		&structs.TxnResult{
			KV: &structs.TxnKVResult{
				DirEnt: &structs.DirEntry{
					Key:   "foo/update",
					Value: []byte("new"),
					RaftIndex: structs.RaftIndex{
						CreateIndex: 5,
						ModifyIndex: 8,
					},
				},
			},
		},
		&structs.TxnResult{}, // get on not/there
		&structs.TxnResult{
			KV: &structs.TxnKVResult{
				DirEnt: &structs.DirEntry{
					Key: "foo/update",
					RaftIndex: structs.RaftIndex{
						CreateIndex: 5,
						ModifyIndex: 8,
					},
				},
			},
		},
		&structs.TxnResult{}, // get on foo/lock before it's created
		&structs.TxnResult{
			KV: &structs.TxnKVResult{
				DirEnt: &structs.DirEntry{
					Key:       "foo/lock",
					Session:   session,
					LockIndex: 1,
					RaftIndex: structs.RaftIndex{
						CreateIndex: 8,
						ModifyIndex: 8,
					},
				},
			},
		},
		&structs.TxnResult{
			KV: &structs.TxnKVResult{
				DirEnt: &structs.DirEntry{
					Key:       "foo/lock",
					Session:   session,
					LockIndex: 1,
					RaftIndex: structs.RaftIndex{
						CreateIndex: 8,
						ModifyIndex: 8,
					},
				},
			},
		},
		&structs.TxnResult{
			KV: &structs.TxnKVResult{
				DirEnt: &structs.DirEntry{
					Key:       "foo/lock",
					LockIndex: 1,
					RaftIndex: structs.RaftIndex{
						CreateIndex: 8,
						ModifyIndex: 8,
					},
				},
			},
		},
		&structs.TxnResult{
			KV: &structs.TxnKVResult{
				DirEnt: &structs.DirEntry{
					Key:       "foo/lock",
					LockIndex: 1,
					RaftIndex: structs.RaftIndex{
						CreateIndex: 8,
						ModifyIndex: 8,
					},
				},
			},
		},
	}
	if len(results) != len(expected) {
		t.Fatalf("bad: %v", results)
	}
	for i, _ := range results {
		if !reflect.DeepEqual(results[i], expected[i]) {
			t.Fatalf("bad %d", i)
		}
	}

	// Pull the resulting state store contents.
	idx, actual, err := s.KVSList("")
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
	for i, _ := range actual {
		if !reflect.DeepEqual(actual[i], entries[i]) {
			t.Fatalf("bad %d", i)
		}
	}
}

func TestStateStore_Txn_KVS_Rollback(t *testing.T) {
	s := testStateStore(t)

	// Create kvs results in the state store.
	testSetKey(t, s, 1, "foo/delete", "bar")
	testSetKey(t, s, 2, "foo/update", "stale")

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
		idx, actual, err := s.KVSList("")
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
		for i, _ := range actual {
			if !reflect.DeepEqual(actual[i], entries[i]) {
				t.Fatalf("bad (%s): op %d: %v != %v", desc, i, *(actual[i]), *(entries[i]))
			}
		}
	}
	verifyStateStore("initial")

	// Set up a transaction that fails every operation.
	ops := structs.TxnOps{
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: structs.KVSCAS,
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
				Verb: structs.KVSLock,
				DirEnt: structs.DirEntry{
					Key:     "foo/lock",
					Session: bogus,
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: structs.KVSUnlock,
				DirEnt: structs.DirEntry{
					Key:     "foo/lock",
					Session: bogus,
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: structs.KVSCheckSession,
				DirEnt: structs.DirEntry{
					Key:     "foo/lock",
					Session: bogus,
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: structs.KVSCheckSession,
				DirEnt: structs.DirEntry{
					Key:     "nope",
					Session: bogus,
				},
			},
		},
		&structs.TxnOp{
			KV: &structs.TxnKVOp{
				Verb: structs.KVSCheckIndex,
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
				Verb: structs.KVSCheckIndex,
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
	results, errors := s.TxnRun(7, ops)
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

func TestStateStore_Txn_Watches(t *testing.T) {
	s := testStateStore(t)

	// Verify that a basic transaction triggers multiple watches. We call
	// the same underlying methods that are called above so this is more
	// of a sanity check.
	verifyWatch(t, s.GetKVSWatch("multi/one"), func() {
		verifyWatch(t, s.GetKVSWatch("multi/two"), func() {
			ops := structs.TxnOps{
				&structs.TxnOp{
					KV: &structs.TxnKVOp{
						Verb: structs.KVSSet,
						DirEnt: structs.DirEntry{
							Key:   "multi/one",
							Value: []byte("one"),
						},
					},
				},
				&structs.TxnOp{
					KV: &structs.TxnKVOp{
						Verb: structs.KVSSet,
						DirEnt: structs.DirEntry{
							Key:   "multi/two",
							Value: []byte("two"),
						},
					},
				},
			}
			results, errors := s.TxnRun(15, ops)
			if len(results) != len(ops) {
				t.Fatalf("bad len: %d != %d", len(results), len(ops))
			}
			if len(errors) != 0 {
				t.Fatalf("bad len: %d != 0", len(errors))
			}
		})
	})

	// Verify that a rolled back transaction doesn't trigger any watches.
	verifyNoWatch(t, s.GetKVSWatch("multi/one"), func() {
		verifyNoWatch(t, s.GetKVSWatch("multi/two"), func() {
			ops := structs.TxnOps{
				&structs.TxnOp{
					KV: &structs.TxnKVOp{
						Verb: structs.KVSSet,
						DirEnt: structs.DirEntry{
							Key:   "multi/one",
							Value: []byte("one-updated"),
						},
					},
				},
				&structs.TxnOp{
					KV: &structs.TxnKVOp{
						Verb: structs.KVSSet,
						DirEnt: structs.DirEntry{
							Key:   "multi/two",
							Value: []byte("two-updated"),
						},
					},
				},
				&structs.TxnOp{
					KV: &structs.TxnKVOp{
						Verb: structs.KVSLock,
						DirEnt: structs.DirEntry{
							Key:   "multi/nope",
							Value: []byte("nope"),
						},
					},
				},
			}
			results, errors := s.TxnRun(16, ops)
			if len(errors) != 1 {
				t.Fatalf("bad len: %d != 1", len(errors))
			}
			if len(results) != 0 {
				t.Fatalf("bad len: %d != 0", len(results))
			}
		})
	})
}
