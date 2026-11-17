// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package security

import (
	"crypto/rand"
	"encoding/base64"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
)

func newGossipKey(t *testing.T) string {
	t.Helper()
	raw := make([]byte, 32)
	_, err := rand.Read(raw)
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(raw)
}

// lanRing returns the LAN keyring response for the datacenter.
func lanRing(t retry.TestingTB, c *api.Client) *api.KeyringResponse {
	rings, err := c.Operator().KeyringList(nil)
	require.NoError(t, err)
	for _, ring := range rings {
		if !ring.WAN {
			return ring
		}
	}
	require.FailNow(t, "no LAN keyring in the response")
	return nil
}

func wanRing(t retry.TestingTB, c *api.Client) *api.KeyringResponse {
	rings, err := c.Operator().KeyringList(nil)
	require.NoError(t, err)
	for _, ring := range rings {
		if ring.WAN {
			return ring
		}
	}
	require.FailNow(t, "no WAN keyring in the response")
	return nil
}

func requireClusterHealthy(t *testing.T, e *env) {
	t.Helper()

	members, err := e.client.Agent().Members(false)
	require.NoError(t, err)
	require.Len(t, members, 2)
	for _, m := range members {
		require.Equal(t, 1, m.Status, "member %s must be alive", m.Name) // serf alive

	}

	// Data written through the client agent is readable through the server.
	_, err = e.client.KV().Put(&api.KVPair{Key: "keyring/check", Value: []byte("ok")}, nil)
	require.NoError(t, err)
	pair, _, err := e.server.KV().Get("keyring/check", nil)
	require.NoError(t, err)
	require.NotNil(t, pair)
}

// TestKeyring_Rotation verifies that the gossip encryption key can be rotated
// on a live cluster: a new key is installed everywhere, made primary, and the
// old key is removed, without the cluster losing members.
func TestKeyring_Rotation(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "")
	op := e.server.Operator()

	var original string

	t.Run("the cluster starts with one key on every node", func(t *testing.T) {
		lan := lanRing(t, e.server)
		require.Equal(t, 2, lan.NumNodes)
		require.Len(t, lan.Keys, 1)
		for key, count := range lan.Keys {
			original = key
			require.Equal(t, 2, count, "the key is installed on both agents")
		}
		require.Equal(t, map[string]int{original: 2}, lan.PrimaryKeys)

		wan := wanRing(t, e.server)
		require.Equal(t, 1, wan.NumNodes, "only servers are in the WAN pool")
		require.Equal(t, 1, wan.Keys[original])

		requireClusterHealthy(t, e)
	})

	replacement := newGossipKey(t)

	t.Run("an installed key is distributed but not yet used", func(t *testing.T) {
		require.NoError(t, op.KeyringInstall(replacement, nil))

		eventually(t, func(r *retry.R) {
			lan := lanRing(r, e.server)
			require.Equal(r, 2, lan.Keys[original])
			require.Equal(r, 2, lan.Keys[replacement], "the new key reaches every node")
			require.Equal(r, map[string]int{original: 2}, lan.PrimaryKeys, "the old key is still primary")
		})
		requireClusterHealthy(t, e)
	})

	t.Run("a key that is not installed cannot be made primary", func(t *testing.T) {
		err := op.KeyringUse(newGossipKey(t), nil)

		require.Error(t, err)
		require.Contains(t, err.Error(), "requested key is not in the keyring")
	})

	t.Run("the new key becomes primary", func(t *testing.T) {
		require.NoError(t, op.KeyringUse(replacement, nil))

		eventually(t, func(r *retry.R) {
			require.Equal(r, map[string]int{replacement: 2}, lanRing(r, e.server).PrimaryKeys)
		})
		requireClusterHealthy(t, e)
	})

	t.Run("the primary key cannot be removed", func(t *testing.T) {
		err := op.KeyringRemove(replacement, nil)

		require.Error(t, err)
		require.Contains(t, err.Error(), "removing the primary key is not allowed")
	})

	t.Run("the old key is removed once it is no longer primary", func(t *testing.T) {
		require.NoError(t, op.KeyringRemove(original, nil))

		eventually(t, func(r *retry.R) {
			lan := lanRing(r, e.server)
			require.Equal(r, map[string]int{replacement: 2}, lan.Keys, "only the new key is left, on every node")
		})
		requireClusterHealthy(t, e)
	})

	t.Run("a malformed key is rejected before anything changes", func(t *testing.T) {
		err := op.KeyringInstall("not-a-valid-key", nil)

		require.Error(t, err)
		require.Contains(t, err.Error(), "illegal base64")
		require.Len(t, lanRing(t, e.server).Keys, 1)
	})
}

// TestKeyring_CLI covers `consul keyring`.
func TestKeyring_CLI(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "")
	replacement := newGossipKey(t)

	shell := func(args string) libcluster.ExecResult { return e.shell(t, "consul keyring "+args) }

	original := func() string {
		for key := range lanRing(t, e.server).PrimaryKeys {
			return key
		}
		return ""
	}()
	require.NotEmpty(t, original)

	keyLine := func(key string) *regexp.Regexp {
		return regexp.MustCompile(regexp.QuoteMeta(key) + `\s+\[(\d+)/(\d+)\]`)
	}

	t.Run("listing shows each key and where it is installed", func(t *testing.T) {
		res := shell("-list")

		require.Equal(t, 0, res.ExitCode, res.Output)
		require.Contains(t, res.Output, "WAN:")
		require.Contains(t, res.Output, "dc1 (LAN):")
		match := keyLine(original).FindStringSubmatch(res.Output[strings.Index(res.Output, "dc1 (LAN):"):])
		require.NotNil(t, match, res.Output)
		require.Equal(t, "2", match[1])
		require.Equal(t, "2", match[2], "installed on 2 of 2 nodes")
	})

	t.Run("installing, using and removing a key", func(t *testing.T) {
		res := shell("-install " + replacement)
		require.Equal(t, 0, res.ExitCode, res.Output)

		eventually(t, func(r *retry.R) {
			require.Equal(r, 2, lanRing(r, e.server).Keys[replacement])
		})

		res = shell("-use " + replacement)
		require.Equal(t, 0, res.ExitCode, res.Output)
		eventually(t, func(r *retry.R) {
			require.Equal(r, map[string]int{replacement: 2}, lanRing(r, e.server).PrimaryKeys)
		})

		res = shell("-remove " + original)
		require.Equal(t, 0, res.ExitCode, res.Output)
		eventually(t, func(r *retry.R) {
			require.Equal(r, map[string]int{replacement: 2}, lanRing(r, e.server).Keys)
		})
	})

	t.Run("failures report which nodes refused", func(t *testing.T) {
		res := shell("-remove " + replacement)

		require.NotEqual(t, 0, res.ExitCode)
		require.Contains(t, res.Output, "removing the primary key is not allowed")
		require.Contains(t, res.Output, "2/2 nodes reported failure")
	})
}
