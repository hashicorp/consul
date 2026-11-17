// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package kv

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
)

func kvSet(key, value string) *api.TxnOp {
	return &api.TxnOp{KV: &api.KVTxnOp{Verb: api.KVSet, Key: key, Value: []byte(value)}}
}

func kvOp(verb api.KVOp, key string) *api.TxnOp {
	return &api.TxnOp{KV: &api.KVTxnOp{Verb: verb, Key: key}}
}

func kvCAS(key, value string, index uint64) *api.TxnOp {
	return &api.TxnOp{KV: &api.KVTxnOp{Verb: api.KVCAS, Key: key, Value: []byte(value), Index: index}}
}

// runTxn applies ops and returns whether the transaction committed, with the
// response carrying results or per-operation errors.
func runTxn(t *testing.T, c *api.Client, ops ...*api.TxnOp) (bool, *api.TxnResponse) {
	t.Helper()
	ok, resp, _, err := c.Txn().Txn(ops, nil)
	require.NoError(t, err)
	return ok, resp
}

func exists(t *testing.T, kv *api.KV, key string) bool {
	t.Helper()
	pair, _, err := kv.Get(key, nil)
	require.NoError(t, err)
	return pair != nil
}

// TestTxn_KV verifies the all-or-nothing contract of /v1/txn for key/value
// operations.
func TestTxn_KV(t *testing.T) {
	t.Parallel()

	e := newEnv(t, envOpts{})
	c := e.client
	kv := c.KV()

	t.Run("operations commit together and return results in order", func(t *testing.T) {
		ok, resp := runTxn(t, c, kvSet("txn/a", "1"), kvSet("txn/b", "2"), kvOp(api.KVGet, "txn/a"), kvOp(api.KVGetTree, "txn/"))

		require.True(t, ok)
		require.Empty(t, resp.Errors)
		require.Len(t, resp.Results, 5, "one result per set and get, and one per key matched by get-tree")
		require.Equal(t, "txn/a", resp.Results[2].KV.Key)
		require.Equal(t, "1", string(resp.Results[2].KV.Value), "a read sees the earlier write in the same transaction")
		require.Equal(t, []string{"txn/a", "txn/b"}, []string{resp.Results[3].KV.Key, resp.Results[4].KV.Key})
		require.Equal(t, "2", string(mustGet(t, kv, "txn/b").Value))
	})

	t.Run("a failing operation rolls everything back", func(t *testing.T) {
		before := mustGet(t, kv, "txn/a")

		ok, resp := runTxn(t, c,
			kvSet("txn/c", "3"),
			kvCAS("txn/a", "changed", before.ModifyIndex-1), // stale index
			kvOp(api.KVDelete, "txn/b"),
		)

		require.False(t, ok)
		require.Len(t, resp.Errors, 1)
		require.Equal(t, 1, resp.Errors[0].OpIndex, "the error names the operation that failed")
		require.Contains(t, resp.Errors[0].What, "index is stale")

		require.False(t, exists(t, kv, "txn/c"), "the earlier set must be undone")
		require.True(t, exists(t, kv, "txn/b"), "the later delete must not run")
		require.Equal(t, "1", string(mustGet(t, kv, "txn/a").Value))
	})

	t.Run("check operations guard the transaction", func(t *testing.T) {
		current := mustGet(t, kv, "txn/a")

		ok, resp := runTxn(t, c,
			&api.TxnOp{KV: &api.KVTxnOp{Verb: api.KVCheckIndex, Key: "txn/a", Index: current.ModifyIndex}},
			kvOp(api.KVCheckNotExists, "txn/never"),
			kvSet("txn/guarded", "ok"),
		)
		require.True(t, ok, "%v", resp.Errors)
		require.True(t, exists(t, kv, "txn/guarded"))

		ok, resp = runTxn(t, c, kvOp(api.KVCheckNotExists, "txn/a"), kvSet("txn/not-written", "x"))
		require.False(t, ok)
		require.Contains(t, resp.Errors[0].What, `key "txn/a" exists`)
		require.False(t, exists(t, kv, "txn/not-written"))

		ok, resp = runTxn(t, c,
			&api.TxnOp{KV: &api.KVTxnOp{Verb: api.KVCheckIndex, Key: "txn/a", Index: current.ModifyIndex - 1}},
			kvSet("txn/not-written", "x"),
		)
		require.False(t, ok)
		require.False(t, exists(t, kv, "txn/not-written"))
	})

	t.Run("deleting trees and keys with check-and-set", func(t *testing.T) {
		ok, _ := runTxn(t, c, kvSet("tree/a", "1"), kvSet("tree/b", "2"), kvSet("keep/c", "3"))
		require.True(t, ok)
		current := mustGet(t, kv, "tree/a")

		ok, _ = runTxn(t, c, &api.TxnOp{KV: &api.KVTxnOp{Verb: api.KVDeleteCAS, Key: "tree/a", Index: current.ModifyIndex - 1}})
		require.False(t, ok, "a stale delete-cas fails")
		require.True(t, exists(t, kv, "tree/a"))

		ok, _ = runTxn(t, c,
			&api.TxnOp{KV: &api.KVTxnOp{Verb: api.KVDeleteCAS, Key: "tree/a", Index: current.ModifyIndex}},
			kvOp(api.KVDeleteTree, "tree/"),
		)
		require.True(t, ok)
		list, _, err := kv.List("tree/", nil)
		require.NoError(t, err)
		require.Empty(t, list)
		require.True(t, exists(t, kv, "keep/c"))
	})

	t.Run("lock, check-session and unlock take part in transactions", func(t *testing.T) {
		sid, _, err := c.Session().Create(&api.SessionEntry{Name: "txn-session", TTL: "30s", Behavior: api.SessionBehaviorRelease}, nil)
		require.NoError(t, err)

		ok, resp := runTxn(t, c,
			&api.TxnOp{KV: &api.KVTxnOp{Verb: api.KVLock, Key: "txn/lock", Value: []byte("held"), Session: sid}},
			&api.TxnOp{KV: &api.KVTxnOp{Verb: api.KVCheckSession, Key: "txn/lock", Session: sid}},
		)
		require.True(t, ok, "%v", resp.Errors)
		require.Equal(t, sid, mustGet(t, kv, "txn/lock").Session)

		other, _, err := c.Session().Create(&api.SessionEntry{Name: "other", TTL: "30s"}, nil)
		require.NoError(t, err)
		ok, _ = runTxn(t, c, &api.TxnOp{KV: &api.KVTxnOp{Verb: api.KVCheckSession, Key: "txn/lock", Session: other}})
		require.False(t, ok, "only the holder passes check-session")

		ok, _ = runTxn(t, c, &api.TxnOp{KV: &api.KVTxnOp{Verb: api.KVUnlock, Key: "txn/lock", Session: sid}})
		require.True(t, ok)
		require.Empty(t, mustGet(t, kv, "txn/lock").Session)
	})

	t.Run("invalid operations are rejected", func(t *testing.T) {
		ok, resp := runTxn(t, c, kvSet("txn/valid", "x"), &api.TxnOp{KV: &api.KVTxnOp{Verb: "explode", Key: "txn/x"}})
		require.False(t, ok)
		require.Contains(t, resp.Errors[0].What, "unknown KV operation")
		require.False(t, exists(t, kv, "txn/valid"))
	})

	t.Run("a transaction may hold at most 128 operations", func(t *testing.T) {
		build := func(n int) []*api.TxnOp {
			ops := make([]*api.TxnOp, n)
			for i := range ops {
				ops[i] = kvSet(fmt.Sprintf("limit/%03d", i), "x")
			}
			return ops
		}

		ok, resp := runTxn(t, c, build(128)...)
		require.True(t, ok, "%v", resp.Errors)

		_, _, _, err := c.Txn().Txn(build(129), nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "too many operations")
	})
}

// TestTxn_Catalog verifies that node, service and check operations commit
// atomically with each other and with key/value operations.
func TestTxn_Catalog(t *testing.T) {
	t.Parallel()

	e := newEnv(t, envOpts{})
	c := e.client

	node := func(verb api.NodeOp, name string) *api.TxnOp {
		return &api.TxnOp{Node: &api.NodeTxnOp{Verb: verb, Node: api.Node{Node: name, Address: "10.9.9.9"}}}
	}
	service := func(verb api.ServiceOp, nodeName, id string) *api.TxnOp {
		return &api.TxnOp{Service: &api.ServiceTxnOp{Verb: verb, Node: nodeName, Service: api.AgentService{ID: id, Service: id, Port: 80}}}
	}
	check := func(verb api.CheckOp, nodeName, id, serviceID, status string) *api.TxnOp {
		return &api.TxnOp{Check: &api.CheckTxnOp{Verb: verb, Check: api.HealthCheck{
			Node: nodeName, CheckID: id, Name: id, Status: status, ServiceID: serviceID,
		}}}
	}

	t.Run("a node, its service and a key are created together", func(t *testing.T) {
		ok, resp := runTxn(t, c, node(api.NodeSet, "txn-node"), service(api.ServiceSet, "txn-node", "txn-svc"), kvSet("txn/with-catalog", "1"))
		require.True(t, ok, "%v", resp.Errors)

		entries, _, err := c.Health().Service("txn-svc", "", false, nil)
		require.NoError(t, err)
		require.Len(t, entries, 1)
		require.Equal(t, "txn-node", entries[0].Node.Node)
		require.Equal(t, "10.9.9.9", entries[0].Node.Address)
		require.True(t, exists(t, c.KV(), "txn/with-catalog"))
	})

	t.Run("a check attaches to a service created in an earlier transaction", func(t *testing.T) {
		ok, resp := runTxn(t, c, check(api.CheckSet, "txn-node", "txn-check", "txn-svc", api.HealthPassing))
		require.True(t, ok, "%v", resp.Errors)

		entries, _, err := c.Health().Service("txn-svc", "", true, nil)
		require.NoError(t, err)
		require.Len(t, entries, 1)

		ok, _ = runTxn(t, c, check(api.CheckSet, "txn-node", "txn-check", "txn-svc", api.HealthCritical))
		require.True(t, ok)
		entries, _, err = c.Health().Service("txn-svc", "", true, nil)
		require.NoError(t, err)
		require.Empty(t, entries, "the service is no longer passing")
	})

	t.Run("reads return catalog objects alongside keys", func(t *testing.T) {
		ok, resp := runTxn(t, c,
			kvOp(api.KVGet, "txn/with-catalog"),
			&api.TxnOp{Node: &api.NodeTxnOp{Verb: api.NodeGet, Node: api.Node{Node: "txn-node"}}},
			&api.TxnOp{Service: &api.ServiceTxnOp{Verb: api.ServiceGet, Node: "txn-node", Service: api.AgentService{ID: "txn-svc", Service: "txn-svc"}}},
			&api.TxnOp{Check: &api.CheckTxnOp{Verb: api.CheckGet, Check: api.HealthCheck{Node: "txn-node", CheckID: "txn-check"}}},
		)
		require.True(t, ok, "%v", resp.Errors)
		require.Len(t, resp.Results, 4)
		require.Equal(t, "txn/with-catalog", resp.Results[0].KV.Key)
		require.Equal(t, "txn-node", resp.Results[1].Node.Node)
		require.Equal(t, 80, resp.Results[2].Service.Port)
		require.Equal(t, api.HealthCritical, resp.Results[3].Check.Status)
	})

	t.Run("a failure rolls back catalog changes too", func(t *testing.T) {
		ok, resp := runTxn(t, c,
			node(api.NodeSet, "rolled-back-node"),
			kvCAS("txn/with-catalog", "x", 1), // stale index
		)
		require.False(t, ok)
		require.NotEmpty(t, resp.Errors)

		got, _, err := c.Catalog().Node("rolled-back-node", nil)
		require.NoError(t, err)
		require.Nil(t, got, "the node must not exist")
	})

	t.Run("a check cannot reference a service that does not exist", func(t *testing.T) {
		ok, resp := runTxn(t, c, node(api.NodeSet, "another-node"), check(api.CheckSet, "another-node", "orphan", "no-such-service", api.HealthPassing))
		require.False(t, ok)
		require.Contains(t, resp.Errors[0].What, "unknown service ID")
	})

	t.Run("deleting a check, service and node", func(t *testing.T) {
		ok, resp := runTxn(t, c, &api.TxnOp{Check: &api.CheckTxnOp{Verb: api.CheckDelete, Check: api.HealthCheck{Node: "txn-node", CheckID: "txn-check"}}})
		require.True(t, ok, "%v", resp.Errors)

		ok, resp = runTxn(t, c, &api.TxnOp{Service: &api.ServiceTxnOp{Verb: api.ServiceDelete, Node: "txn-node", Service: api.AgentService{ID: "txn-svc", Service: "txn-svc"}}})
		require.True(t, ok, "%v", resp.Errors)

		ok, resp = runTxn(t, c, node(api.NodeDelete, "txn-node"))
		require.True(t, ok, "%v", resp.Errors)

		got, _, err := c.Catalog().Node("txn-node", nil)
		require.NoError(t, err)
		require.Nil(t, got)
	})
}

// TestTxn_ACLs verifies that a transaction is authorized per operation and is
// all-or-nothing: one denied operation prevents the whole transaction.
func TestTxn_ACLs(t *testing.T) {
	t.Parallel()

	e := newEnv(t, envOpts{acl: true})
	admin := e.client.KV()
	put(t, admin, "app/existing", "v")
	put(t, admin, "other/secret", "s")

	writer := e.newToken(t, "txn-app-writer", `key_prefix "app/" { policy = "write" }`)
	reader := e.newToken(t, "txn-app-reader", `key_prefix "app/" { policy = "read" }`)

	t.Run("a transaction within the token's policy commits", func(t *testing.T) {
		ok, resp := runTxn(t, writer, kvSet("app/a", "1"), kvSet("app/b", "2"))
		require.True(t, ok, "%v", resp.Errors)
		require.Equal(t, "2", string(mustGet(t, admin, "app/b").Value))
	})

	t.Run("one denied operation stops the whole transaction", func(t *testing.T) {
		ok, resp := runTxn(t, writer, kvSet("app/should-not-exist", "1"), kvSet("other/forbidden", "1"))

		require.False(t, ok)
		require.Len(t, resp.Errors, 1)
		require.Equal(t, 1, resp.Errors[0].OpIndex)
		require.Contains(t, resp.Errors[0].What, "Permission denied")
		require.False(t, exists(t, admin, "app/should-not-exist"), "the permitted operation must not be applied")
		require.False(t, exists(t, admin, "other/forbidden"))
	})

	t.Run("a read-only token cannot write", func(t *testing.T) {
		ok, resp := runTxn(t, reader, kvSet("app/nope", "1"))
		require.False(t, ok)
		require.Contains(t, resp.Errors[0].What, "Permission denied")

		ok, resp = runTxn(t, reader, kvOp(api.KVGet, "app/existing"))
		require.True(t, ok, "%v", resp.Errors)
		require.Equal(t, "v", string(resp.Results[0].KV.Value))
	})

	t.Run("reads of keys the token cannot see are filtered out", func(t *testing.T) {
		ok, resp := runTxn(t, reader, kvOp(api.KVGet, "other/secret"))

		require.True(t, ok)
		require.Empty(t, resp.Results, "a hidden key is simply absent from the results")
		require.Empty(t, resp.Errors)
	})
}
