// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package wanfed

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

func policyNames(t retry.TestingTB, c *api.Client) []string {
	list, _, err := c.ACL().PolicyList(nil)
	require.NoError(t, err)
	var names []string
	for _, p := range list {
		names = append(names, p.Name)
	}
	return names
}

func roleNames(t retry.TestingTB, c *api.Client) []string {
	list, _, err := c.ACL().RoleList(nil)
	require.NoError(t, err)
	var names []string
	for _, r := range list {
		names = append(names, r.Name)
	}
	return names
}

func tokenAccessors(t retry.TestingTB, c *api.Client) []string {
	list, _, err := c.ACL().TokenList(nil)
	require.NoError(t, err)
	var ids []string
	for _, tok := range list {
		ids = append(ids, tok.AccessorID)
	}
	return ids
}

// TestWanFed_ACLReplication verifies that policies, roles and tokens defined
// in the primary datacenter are replicated to the secondary and enforced there,
// that updates and deletions follow, and that ACL writes made in the secondary
// are forwarded to the primary.
func TestWanFed_ACLReplication(t *testing.T) {
	t.Parallel()

	f := newACLFederation(t)

	var (
		policy *api.ACLPolicy
		role   *api.ACLRole
		token  *api.ACLToken
	)

	t.Run("replication is running in the secondary", func(t *testing.T) {
		eventually(t, func(r *retry.R) {
			status, _, err := f.sc.ACL().Replication(nil)
			require.NoError(r, err)
			require.True(r, status.Enabled)
			require.True(r, status.Running, status.LastErrorMessage)
			require.Equal(r, primaryDC, status.SourceDatacenter)
			require.Equal(r, "tokens", status.ReplicationType, "policies, roles and tokens are replicated")
		})
	})

	t.Run("policies, roles and tokens created in the primary appear in the secondary", func(t *testing.T) {
		var err error
		policy, _, err = f.pc.ACL().PolicyCreate(&api.ACLPolicy{
			Name:  "app-kv-reader",
			Rules: `key_prefix "app/" { policy = "read" }`,
		}, nil)
		require.NoError(t, err)

		role, _, err = f.pc.ACL().RoleCreate(&api.ACLRole{
			Name:     "app-reader-role",
			Policies: []*api.ACLRolePolicyLink{{Name: policy.Name}},
		}, nil)
		require.NoError(t, err)

		token, _, err = f.pc.ACL().TokenCreate(&api.ACLToken{
			Description: "replicated token",
			Roles:       []*api.ACLTokenRoleLink{{Name: role.Name}},
		}, nil)
		require.NoError(t, err)

		eventually(t, func(r *retry.R) {
			require.Contains(r, policyNames(r, f.sc), policy.Name)
			require.Contains(r, roleNames(r, f.sc), role.Name)
			require.Contains(r, tokenAccessors(r, f.sc), token.AccessorID)

			got, _, err := f.sc.ACL().TokenRead(token.AccessorID, nil)
			require.NoError(r, err)
			require.Equal(r, token.SecretID, got.SecretID, "the secret is replicated, so the token works everywhere")
			require.Len(r, got.Roles, 1)
			require.Equal(r, role.Name, got.Roles[0].Name)
		})
	})

	t.Run("the replicated token is enforced in the secondary", func(t *testing.T) {
		_, err := f.sc.KV().Put(&api.KVPair{Key: "app/config", Value: []byte("v1")}, nil)
		require.NoError(t, err)
		_, err = f.sc.KV().Put(&api.KVPair{Key: "other/config", Value: []byte("v1")}, nil)
		require.NoError(t, err)

		secondaryAsToken := f.tokenClient(t, f.secondary, token.SecretID)
		eventually(t, func(r *retry.R) {
			pair, _, err := secondaryAsToken.KV().Get("app/config", nil)
			require.NoError(r, err)
			require.NotNil(r, pair)
			require.Equal(r, "v1", string(pair.Value))
		})

		_, _, err = secondaryAsToken.KV().Get("other/config", nil)
		require.Error(t, err, "the policy only grants app/")
		require.Contains(t, err.Error(), "403")

		_, err = secondaryAsToken.KV().Put(&api.KVPair{Key: "app/config", Value: []byte("v2")}, nil)
		require.Error(t, err, "the policy is read-only")
	})

	t.Run("policy updates replicate and change what the token may do", func(t *testing.T) {
		updated := *policy
		updated.Rules = `key_prefix "other/" { policy = "read" }`
		_, _, err := f.pc.ACL().PolicyUpdate(&updated, nil)
		require.NoError(t, err)

		secondaryAsToken := f.tokenClient(t, f.secondary, token.SecretID)
		eventually(t, func(r *retry.R) {
			pair, _, err := secondaryAsToken.KV().Get("other/config", nil)
			require.NoError(r, err)
			require.NotNil(r, pair)
		})

		_, _, err = secondaryAsToken.KV().Get("app/config", nil)
		require.Error(t, err, "the old grant must be gone")
	})

	t.Run("a token created in the secondary is forwarded to the primary", func(t *testing.T) {
		created, _, err := f.sc.ACL().TokenCreate(&api.ACLToken{
			Description: "created in the secondary",
			Policies:    []*api.ACLTokenPolicyLink{{Name: policy.Name}},
		}, nil)
		require.NoError(t, err)

		eventually(t, func(r *retry.R) {
			require.Contains(r, tokenAccessors(r, f.pc), created.AccessorID)
			require.Contains(r, tokenAccessors(r, f.sc), created.AccessorID)
		})
	})

	t.Run("deleting the token in the primary revokes it in the secondary", func(t *testing.T) {
		_, err := f.pc.ACL().TokenDelete(token.AccessorID, nil)
		require.NoError(t, err)

		eventually(t, func(r *retry.R) {
			require.NotContains(r, tokenAccessors(r, f.sc), token.AccessorID)
		})

		secondaryAsToken := f.tokenClient(t, f.secondary, token.SecretID)
		eventually(t, func(r *retry.R) {
			_, _, err := secondaryAsToken.KV().Get("other/config", nil)
			require.Error(r, err, "a deleted token must stop working")
		})
	})

	t.Run("deleting the role and policy replicates", func(t *testing.T) {
		_, err := f.pc.ACL().RoleDelete(role.ID, nil)
		require.NoError(t, err)
		_, err = f.pc.ACL().PolicyDelete(policy.ID, nil)
		require.NoError(t, err)

		eventually(t, func(r *retry.R) {
			require.NotContains(r, roleNames(r, f.sc), role.Name)
			require.NotContains(r, policyNames(r, f.sc), policy.Name)
		})
	})
}
