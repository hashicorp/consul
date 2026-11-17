// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package kv

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
)

// newToken creates a token holding one policy and returns a client to the
// client agent that authenticates with it.
func (e *env) newToken(t *testing.T, name, rules string) *api.Client {
	t.Helper()

	_, _, err := e.server.ACL().PolicyCreate(&api.ACLPolicy{Name: name, Rules: rules}, nil)
	require.NoError(t, err)
	token, _, err := e.server.ACL().TokenCreate(&api.ACLToken{
		Description: name,
		Policies:    []*api.ACLTokenPolicyLink{{Name: name}},
	}, nil)
	require.NoError(t, err)
	return e.tokenClient(t, token.SecretID)
}

func requireDenied(t *testing.T, err error, msg string) {
	t.Helper()
	require.Error(t, err, msg)
	require.Contains(t, err.Error(), "403", msg)
}

// TestKV_ACLs verifies key/value access control with default-deny ACLs: access
// is granted per key or per prefix, reads and writes are separate, and listings
// only show keys the token may read.
func TestKV_ACLs(t *testing.T) {
	t.Parallel()

	e := newEnv(t, envOpts{acl: true})
	admin := e.client.KV()
	for _, key := range []string{"app/one", "app/two", "other/secret"} {
		put(t, admin, key, "v:"+key)
	}

	prefixWriter := e.newToken(t, "app-writer", `key_prefix "app/" { policy = "write" }`).KV()
	prefixReader := e.newToken(t, "app-reader", `key_prefix "app/" { policy = "read" }`).KV()
	singleKey := e.newToken(t, "one-key", `key "app/one" { policy = "write" }`).KV()
	unrelated := e.newToken(t, "unrelated", `key_prefix "zzz/" { policy = "read" }`).KV()

	t.Run("reads follow the policy", func(t *testing.T) {
		for name, kv := range map[string]*api.KV{"prefix writer": prefixWriter, "prefix reader": prefixReader, "single key": singleKey} {
			pair, _, err := kv.Get("app/one", nil)
			require.NoError(t, err, name)
			require.Equal(t, "v:app/one", string(pair.Value), name)
		}

		_, _, err := unrelated.Get("app/one", nil)
		requireDenied(t, err, "a token for another prefix")

		for name, kv := range map[string]*api.KV{"prefix writer": prefixWriter, "prefix reader": prefixReader, "single key": singleKey} {
			_, _, err := kv.Get("other/secret", nil)
			requireDenied(t, err, name+" reading outside its policy")
		}
	})

	t.Run("writes need write access", func(t *testing.T) {
		_, err := prefixWriter.Put(&api.KVPair{Key: "app/new", Value: []byte("x")}, nil)
		require.NoError(t, err)
		_, err = singleKey.Put(&api.KVPair{Key: "app/one", Value: []byte("updated")}, nil)
		require.NoError(t, err)

		_, err = prefixReader.Put(&api.KVPair{Key: "app/one", Value: []byte("nope")}, nil)
		requireDenied(t, err, "a read-only token")
		_, err = singleKey.Put(&api.KVPair{Key: "app/two", Value: []byte("nope")}, nil)
		requireDenied(t, err, "a single-key token writing another key")
		_, err = prefixWriter.Put(&api.KVPair{Key: "other/secret", Value: []byte("nope")}, nil)
		requireDenied(t, err, "writing outside the prefix")

		require.Equal(t, "updated", string(mustGet(t, admin, "app/one").Value))
		require.Equal(t, "v:other/secret", string(mustGet(t, admin, "other/secret").Value), "denied writes change nothing")
	})

	t.Run("check-and-set follows write access", func(t *testing.T) {
		current := mustGet(t, admin, "app/two")

		_, _, err := prefixReader.CAS(&api.KVPair{Key: "app/two", Value: []byte("x"), ModifyIndex: current.ModifyIndex}, nil)
		requireDenied(t, err, "a read-only token")

		ok, _, err := prefixWriter.CAS(&api.KVPair{Key: "app/two", Value: []byte("cas-ok"), ModifyIndex: current.ModifyIndex}, nil)
		require.NoError(t, err)
		require.True(t, ok)
	})

	t.Run("listings only include readable keys", func(t *testing.T) {
		list, _, err := prefixReader.List("app/", nil)
		require.NoError(t, err)
		require.Equal(t, []string{"app/new", "app/one", "app/two"}, keysOf(list))

		list, _, err = singleKey.List("app/", nil)
		require.NoError(t, err)
		require.Equal(t, []string{"app/one"}, keysOf(list), "the other keys are filtered out")

		keys, _, err := prefixReader.Keys("", "", nil)
		require.NoError(t, err)
		require.Equal(t, []string{"app/new", "app/one", "app/two"}, keys, "the root listing hides what the token cannot read")

		list, _, err = unrelated.List("app/", nil)
		require.NoError(t, err)
		require.Empty(t, list)
	})

	t.Run("deleting needs write access", func(t *testing.T) {
		_, err := prefixReader.Delete("app/new", nil)
		requireDenied(t, err, "a read-only token")
		_, err = singleKey.Delete("app/new", nil)
		requireDenied(t, err, "a single-key token deleting another key")

		_, err = singleKey.DeleteTree("app/", nil)
		requireDenied(t, err, "a tree delete needs write on the whole prefix")
		require.NotNil(t, mustGet(t, admin, "app/one"), "a denied tree delete must not delete anything")

		_, err = prefixWriter.Delete("app/new", nil)
		require.NoError(t, err)
		_, err = prefixWriter.DeleteTree("app/", nil)
		require.NoError(t, err)
		list, _, err := admin.List("app/", nil)
		require.NoError(t, err)
		require.Empty(t, list)
	})
}
