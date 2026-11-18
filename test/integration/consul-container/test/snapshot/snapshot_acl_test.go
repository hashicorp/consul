// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package snapshot

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

func newPolicyToken(t *testing.T, c *api.Client, name, rules string) *api.ACLToken {
	t.Helper()
	_, _, err := c.ACL().PolicyCreate(&api.ACLPolicy{Name: name, Rules: rules}, nil)
	require.NoError(t, err)
	token, _, err := c.ACL().TokenCreate(&api.ACLToken{
		Description: name,
		Policies:    []*api.ACLTokenPolicyLink{{Name: name}},
	}, nil)
	require.NoError(t, err)
	return token
}

// TestSnapshot_ACL verifies who may take and restore snapshots when ACLs are
// enabled, and that a snapshot carries the ACL data with it.
func TestSnapshot_ACL(t *testing.T) {
	e := newEnv(t, true)
	snap := e.file(t, "acl.snap")

	operator := newPolicyToken(t, e.admin, "operator-only", `operator = "write"`)
	kvReader := newPolicyToken(t, e.admin, "kv-reader", `key_prefix "" { policy = "read" }`)
	_, err := e.admin.KV().Put(&api.KVPair{Key: "data", Value: []byte("1")}, nil)
	require.NoError(t, err)

	out, code := e.cli(t, e.mgmtToken, "snapshot", "save", snap)
	require.Equal(t, 0, code, out)

	t.Run("without acl:write a snapshot cannot be saved or restored", func(t *testing.T) {
		for name, token := range map[string]string{
			"no token":                    "",
			"a token with operator:write": operator.SecretID,
			"a token with key access":     kvReader.SecretID,
		} {
			out, code := e.cli(t, token, "snapshot", "save", e.file(t, "denied.snap"))
			require.NotEqual(t, 0, code, name)
			require.Contains(t, out, "403", name)
			require.Contains(t, out, "lacks permission 'acl:write'", name)

			out, code = e.cli(t, token, "snapshot", "restore", snap)
			require.NotEqual(t, 0, code, name)
			require.Contains(t, out, "Error restoring snapshot", name)
		}

		_, _, err := e.tokenClient(t, "").Snapshot().Save(nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "403")
	})

	t.Run("a token with acl:write can save and inspect", func(t *testing.T) {
		admin := newPolicyToken(t, e.admin, "acl-admin", `acl = "write"`)

		out, code := e.cli(t, admin.SecretID, "snapshot", "save", e.file(t, "by-acl-admin.snap"))
		require.Equal(t, 0, code, out)
		require.Contains(t, out, "Saved and verified snapshot")

		out, code = e.cli(t, "", "snapshot", "inspect", e.file(t, "by-acl-admin.snap"))
		require.Equal(t, 0, code, "inspect only reads the file: %s", out)
	})

	t.Run("a snapshot carries ACL policies and tokens", func(t *testing.T) {
		// Undo and add ACL data after the snapshot.
		_, err := e.admin.ACL().TokenDelete(kvReader.AccessorID, nil)
		require.NoError(t, err)
		later := newPolicyToken(t, e.admin, "created-later", `key_prefix "" { policy = "write" }`)
		_, err = e.admin.KV().Delete("data", nil)
		require.NoError(t, err)

		_, _, err = e.tokenClient(t, kvReader.SecretID).KV().Get("data", nil)
		require.Error(t, err, "the deleted token no longer works")

		out, code := e.cli(t, e.mgmtToken, "snapshot", "restore", snap)
		require.Equal(t, 0, code, out)

		eventually(t, func(r *retry.R) {
			pair, _, err := e.tokenClient(t, kvReader.SecretID).KV().Get("data", nil)
			require.NoError(r, err, "the token that was deleted after the snapshot works again")
			require.NotNil(r, pair)
			require.Equal(r, "1", string(pair.Value))
		})

		_, _, err = e.tokenClient(t, later.SecretID).KV().Get("data", nil)
		require.Error(t, err, "a token created after the snapshot does not survive a restore")

		policy, _, err := e.admin.ACL().PolicyReadByName("created-later", nil)
		require.NoError(t, err)
		require.Nil(t, policy)
	})

	t.Run("the management token keeps working after a restore", func(t *testing.T) {
		snapshot, _, err := e.admin.Snapshot().Save(nil)
		require.NoError(t, err)
		defer snapshot.Close()
		data, err := io.ReadAll(snapshot)
		require.NoError(t, err)
		require.NotEmpty(t, data)
	})
}
