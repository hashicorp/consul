// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package kv

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

func put(t *testing.T, kv *api.KV, key, value string) {
	t.Helper()
	_, err := kv.Put(&api.KVPair{Key: key, Value: []byte(value)}, nil)
	require.NoError(t, err)
}

func mustGet(t *testing.T, kv *api.KV, key string) *api.KVPair {
	t.Helper()
	pair, _, err := kv.Get(key, nil)
	require.NoError(t, err)
	require.NotNil(t, pair, "key %q should exist", key)
	return pair
}

func keysOf(pairs api.KVPairs) []string {
	var keys []string
	for _, p := range pairs {
		keys = append(keys, p.Key)
	}
	return keys
}

// TestKV_CRUD covers the basic key/value contract through the client agent:
// writes, reads, overwrites, listing, hierarchy, special characters, size
// limits and deletion.
func TestKV_CRUD(t *testing.T) {
	t.Parallel()

	e := newEnv(t, envOpts{})
	kv := e.client.KV()

	t.Run("a written value is read back with its flags and indexes", func(t *testing.T) {
		_, err := kv.Put(&api.KVPair{Key: "app/greeting", Value: []byte("hello"), Flags: 42}, nil)
		require.NoError(t, err)

		pair, meta, err := kv.Get("app/greeting", nil)
		require.NoError(t, err)
		require.NotNil(t, pair)
		require.Equal(t, "app/greeting", pair.Key)
		require.Equal(t, "hello", string(pair.Value))
		require.EqualValues(t, 42, pair.Flags)
		require.Zero(t, pair.LockIndex)
		require.Empty(t, pair.Session)
		require.NotZero(t, pair.CreateIndex)
		require.Equal(t, pair.CreateIndex, pair.ModifyIndex, "a new key is created and modified at the same index")
		require.True(t, meta.KnownLeader)
	})

	t.Run("overwriting keeps the creation index and advances the modify index", func(t *testing.T) {
		before := mustGet(t, kv, "app/greeting")

		_, err := kv.Put(&api.KVPair{Key: "app/greeting", Value: []byte("hello again")}, nil)
		require.NoError(t, err)

		after := mustGet(t, kv, "app/greeting")
		require.Equal(t, "hello again", string(after.Value))
		require.Zero(t, after.Flags, "flags are replaced by the write")
		require.Equal(t, before.CreateIndex, after.CreateIndex)
		require.Greater(t, after.ModifyIndex, before.ModifyIndex)
	})

	t.Run("a missing key is not an error", func(t *testing.T) {
		pair, meta, err := kv.Get("app/does-not-exist", nil)
		require.NoError(t, err)
		require.Nil(t, pair)
		require.NotZero(t, meta.LastIndex, "a miss still reports an index to block on")
	})

	t.Run("an empty value is stored", func(t *testing.T) {
		put(t, kv, "app/empty", "")

		pair := mustGet(t, kv, "app/empty")
		require.Empty(t, pair.Value)
	})

	t.Run("keys with spaces and unicode round trip", func(t *testing.T) {
		for _, key := range []string{"special/with space/key", "special/café/ключ", "special/a+b=c"} {
			put(t, kv, key, "v:"+key)
			require.Equal(t, "v:"+key, string(mustGet(t, kv, key).Value), key)
		}
	})

	t.Run("binary values survive", func(t *testing.T) {
		value := []byte{0x00, 0x01, 0xfe, 0xff, '\n', 0x00}
		_, err := kv.Put(&api.KVPair{Key: "app/binary", Value: value}, nil)
		require.NoError(t, err)
		require.True(t, bytes.Equal(value, mustGet(t, kv, "app/binary").Value))
	})

	t.Run("listing and hierarchy", func(t *testing.T) {
		for _, key := range []string{"tree/a", "tree/b/c", "tree/b/d", "tree/bar", "tree/e"} {
			put(t, kv, key, "1")
		}

		list, _, err := kv.List("tree/", nil)
		require.NoError(t, err)
		require.Equal(t, []string{"tree/a", "tree/b/c", "tree/b/d", "tree/bar", "tree/e"}, keysOf(list), "recursive listings are sorted")

		keys, _, err := kv.Keys("tree/", "", nil)
		require.NoError(t, err)
		require.Equal(t, []string{"tree/a", "tree/b/c", "tree/b/d", "tree/bar", "tree/e"}, keys)

		folded, _, err := kv.Keys("tree/", "/", nil)
		require.NoError(t, err)
		require.Equal(t, []string{"tree/a", "tree/b/", "tree/bar", "tree/e"}, folded, "a separator folds deeper keys into one entry")

		missing, _, err := kv.List("nothing-here/", nil)
		require.NoError(t, err)
		require.Empty(t, missing)

		keysMissing, _, err := kv.Keys("nothing-here/", "", nil)
		require.NoError(t, err)
		require.Empty(t, keysMissing)
	})

	t.Run("a prefix matches by string, not by path segment", func(t *testing.T) {
		list, _, err := kv.List("tree/b", nil)
		require.NoError(t, err)
		require.Equal(t, []string{"tree/b/c", "tree/b/d", "tree/bar"}, keysOf(list), `"tree/b" also matches "tree/bar"`)
	})

	t.Run("deleting a single key and a missing key", func(t *testing.T) {
		put(t, kv, "gone/one", "1")

		_, err := kv.Delete("gone/one", nil)
		require.NoError(t, err)
		pair, _, err := kv.Get("gone/one", nil)
		require.NoError(t, err)
		require.Nil(t, pair)

		_, err = kv.Delete("gone/one", nil)
		require.NoError(t, err, "deleting a missing key is idempotent")
	})

	t.Run("deleting a tree removes every key under the prefix string", func(t *testing.T) {
		_, err := kv.DeleteTree("tree/b", nil)
		require.NoError(t, err)

		list, _, err := kv.List("tree/", nil)
		require.NoError(t, err)
		require.Equal(t, []string{"tree/a", "tree/e"}, keysOf(list))
	})

	t.Run("values up to the size limit are accepted and larger ones are refused", func(t *testing.T) {
		const limit = 512 * 1024

		_, err := kv.Put(&api.KVPair{Key: "size/at-limit", Value: bytes.Repeat([]byte("x"), limit)}, nil)
		require.NoError(t, err)
		require.Len(t, mustGet(t, kv, "size/at-limit").Value, limit)

		_, err = kv.Put(&api.KVPair{Key: "size/over-limit", Value: bytes.Repeat([]byte("x"), limit+1)}, nil)
		require.Error(t, err)
		pair, _, err := kv.Get("size/over-limit", nil)
		require.NoError(t, err)
		require.Nil(t, pair, "a refused write must not leave a key behind")
	})
}

// TestKV_CheckAndSet verifies optimistic concurrency: create-if-absent,
// update-if-unchanged and delete-if-unchanged.
func TestKV_CheckAndSet(t *testing.T) {
	t.Parallel()

	e := newEnv(t, envOpts{})
	kv := e.client.KV()

	t.Run("index 0 creates a key only when it does not exist", func(t *testing.T) {
		ok, _, err := kv.CAS(&api.KVPair{Key: "cas/create", Value: []byte("first")}, nil)
		require.NoError(t, err)
		require.True(t, ok)

		ok, _, err = kv.CAS(&api.KVPair{Key: "cas/create", Value: []byte("second")}, nil)
		require.NoError(t, err)
		require.False(t, ok, "the key exists, so create-if-absent must fail")
		require.Equal(t, "first", string(mustGet(t, kv, "cas/create").Value))
	})

	t.Run("an update succeeds only with the current modify index", func(t *testing.T) {
		current := mustGet(t, kv, "cas/create")

		ok, _, err := kv.CAS(&api.KVPair{Key: "cas/create", Value: []byte("stale"), ModifyIndex: current.ModifyIndex - 1}, nil)
		require.NoError(t, err)
		require.False(t, ok)
		require.Equal(t, "first", string(mustGet(t, kv, "cas/create").Value))

		ok, _, err = kv.CAS(&api.KVPair{Key: "cas/create", Value: []byte("updated"), ModifyIndex: current.ModifyIndex}, nil)
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, "updated", string(mustGet(t, kv, "cas/create").Value))
	})

	t.Run("a modify index cannot be reused after the key changed", func(t *testing.T) {
		current := mustGet(t, kv, "cas/create")
		put(t, kv, "cas/create", "changed-elsewhere")

		ok, _, err := kv.CAS(&api.KVPair{Key: "cas/create", Value: []byte("lost update"), ModifyIndex: current.ModifyIndex}, nil)
		require.NoError(t, err)
		require.False(t, ok, "a concurrent writer must win")
		require.Equal(t, "changed-elsewhere", string(mustGet(t, kv, "cas/create").Value))
	})

	t.Run("deleting is conditional on the modify index too", func(t *testing.T) {
		current := mustGet(t, kv, "cas/create")

		ok, _, err := kv.DeleteCAS(&api.KVPair{Key: "cas/create", ModifyIndex: current.ModifyIndex - 1}, nil)
		require.NoError(t, err)
		require.False(t, ok)
		mustGet(t, kv, "cas/create")

		ok, _, err = kv.DeleteCAS(&api.KVPair{Key: "cas/create", ModifyIndex: current.ModifyIndex}, nil)
		require.NoError(t, err)
		require.True(t, ok)
		pair, _, err := kv.Get("cas/create", nil)
		require.NoError(t, err)
		require.Nil(t, pair)
	})
}

// TestKV_ConsistencyModes verifies reads can be served by either agent, and the
// stale and consistent read modes.
func TestKV_ConsistencyModes(t *testing.T) {
	t.Parallel()

	e := newEnv(t, envOpts{})
	put(t, e.server.KV(), "modes/key", "v1")

	t.Run("the client and server agents agree", func(t *testing.T) {
		eventually(t, func(r *retry.R) {
			fromClient, _, err := e.client.KV().Get("modes/key", nil)
			require.NoError(r, err)
			require.NotNil(r, fromClient)

			fromServer, _, err := e.server.KV().Get("modes/key", nil)
			require.NoError(r, err)
			require.NotNil(r, fromServer)

			require.Equal(r, fromServer.ModifyIndex, fromClient.ModifyIndex)
			require.Equal(r, "v1", string(fromClient.Value))
		})
	})

	t.Run("stale reads are served by any server and report leader knowledge", func(t *testing.T) {
		pair, meta, err := e.server.KV().Get("modes/key", &api.QueryOptions{AllowStale: true})
		require.NoError(t, err)
		require.NotNil(t, pair)
		require.True(t, meta.KnownLeader)
	})

	t.Run("consistent reads are accepted", func(t *testing.T) {
		pair, _, err := e.client.KV().Get("modes/key", &api.QueryOptions{RequireConsistent: true})
		require.NoError(t, err)
		require.NotNil(t, pair)
	})

	t.Run("stale and consistent together are contradictory", func(t *testing.T) {
		_, _, err := e.client.KV().Get("modes/key", &api.QueryOptions{AllowStale: true, RequireConsistent: true})
		require.Error(t, err)
		require.Contains(t, err.Error(), "conflicting semantics")
	})
}
