// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package acl

import (
	"testing"
	"time"

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

func tokenAccessors(t retry.TestingTB, c *api.Client) []string {
	list, _, err := c.ACL().TokenList(nil)
	require.NoError(t, err)
	var ids []string
	for _, tok := range list {
		ids = append(ids, tok.AccessorID)
	}
	return ids
}

// TestACL_PolicyLifecycle covers creating, reading, updating, listing and
// deleting policies, and the validation of their names and rules.
func TestACL_PolicyLifecycle(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "", "")
	acl := e.admin.ACL()
	var policy *api.ACLPolicy

	t.Run("a policy is created and read back by ID and by name", func(t *testing.T) {
		var err error
		policy, _, err = acl.PolicyCreate(&api.ACLPolicy{
			Name:        "web-writer",
			Description: "lets web register itself",
			Rules:       `service "web" { policy = "write" }`,
		}, nil)
		require.NoError(t, err)
		require.NotEmpty(t, policy.ID)

		byID, _, err := acl.PolicyRead(policy.ID, nil)
		require.NoError(t, err)
		byName, _, err := acl.PolicyReadByName("web-writer", nil)
		require.NoError(t, err)

		for name, got := range map[string]*api.ACLPolicy{"by id": byID, "by name": byName} {
			require.Equal(t, policy.ID, got.ID, name)
			require.Equal(t, "lets web register itself", got.Description, name)
			require.Contains(t, got.Rules, `service "web"`, name)
		}
	})

	t.Run("listing includes the built-in global-management policy", func(t *testing.T) {
		names := policyNames(t, e.admin)
		require.Contains(t, names, "global-management")
		require.Contains(t, names, "web-writer")
	})

	t.Run("updating changes the rules and the hash", func(t *testing.T) {
		before, _, err := acl.PolicyRead(policy.ID, nil)
		require.NoError(t, err)

		updated := *before
		updated.Rules = `service "web" { policy = "read" }`
		_, _, err = acl.PolicyUpdate(&updated, nil)
		require.NoError(t, err)

		after, _, err := acl.PolicyRead(policy.ID, nil)
		require.NoError(t, err)
		require.Contains(t, after.Rules, `policy = "read"`)
		require.NotEqual(t, before.Hash, after.Hash)
		require.Greater(t, after.ModifyIndex, before.ModifyIndex)
	})

	t.Run("invalid policies are rejected", func(t *testing.T) {
		for name, tc := range map[string]struct {
			policy  *api.ACLPolicy
			wantErr string
		}{
			"a duplicate name":           {&api.ACLPolicy{Name: "web-writer"}, "already exists"},
			"the reserved name":          {&api.ACLPolicy{Name: "global-management"}, "already exists"},
			"no name":                    {&api.ACLPolicy{}, "no Name is set"},
			"invalid characters in name": {&api.ACLPolicy{Name: "bad name!"}, "invalid Name"},
			"rules that do not parse":    {&api.ACLPolicy{Name: "bad-hcl", Rules: "this is not { hcl"}, "Failed to parse ACL rules"},
			"an unknown policy level":    {&api.ACLPolicy{Name: "bad-level", Rules: `service "web" { policy = "bogus" }`}, "Invalid service policy"},
		} {
			_, _, err := acl.PolicyCreate(tc.policy, nil)
			require.Error(t, err, name)
			require.Contains(t, err.Error(), tc.wantErr, name)
		}
	})

	t.Run("the global-management policy is immutable", func(t *testing.T) {
		const id = "00000000-0000-0000-0000-000000000001"

		_, err := acl.PolicyDelete(id, nil)
		require.Error(t, err, "the built-in policy cannot be deleted")

		_, _, err = acl.PolicyUpdate(&api.ACLPolicy{ID: id, Name: "global-management", Rules: ""}, nil)
		require.Error(t, err, "its rules cannot be changed")

		got, _, err := acl.PolicyRead(id, nil)
		require.NoError(t, err)
		require.Equal(t, "global-management", got.Name)
	})

	t.Run("deleting a policy removes it, and the delete is not repeatable", func(t *testing.T) {
		_, err := acl.PolicyDelete(policy.ID, nil)
		require.NoError(t, err)

		_, _, err = acl.PolicyRead(policy.ID, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "404")

		_, err = acl.PolicyDelete(policy.ID, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "404")
	})
}

// TestACL_RoleLifecycle covers roles, which bundle policies and identities so
// that many tokens can share them.
func TestACL_RoleLifecycle(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "", "")
	acl := e.admin.ACL()

	_, _, err := acl.PolicyCreate(&api.ACLPolicy{Name: "role-policy", Rules: `key_prefix "app/" { policy = "read" }`}, nil)
	require.NoError(t, err)

	var role *api.ACLRole

	t.Run("a role bundles policies and service identities", func(t *testing.T) {
		role, _, err = acl.RoleCreate(&api.ACLRole{
			Name:              "app-role",
			Description:       "reads app config and runs db",
			Policies:          []*api.ACLRolePolicyLink{{Name: "role-policy"}},
			ServiceIdentities: []*api.ACLServiceIdentity{{ServiceName: "db"}},
		}, nil)
		require.NoError(t, err)

		got, _, err := acl.RoleReadByName("app-role", nil)
		require.NoError(t, err)
		require.Equal(t, role.ID, got.ID)
		require.Len(t, got.Policies, 1)
		require.Equal(t, "role-policy", got.Policies[0].Name)
		require.Len(t, got.ServiceIdentities, 1)
		require.Equal(t, "db", got.ServiceIdentities[0].ServiceName)
	})

	t.Run("a role can be renamed", func(t *testing.T) {
		updated := *role
		updated.Name = "app-role-renamed"
		_, _, err := acl.RoleUpdate(&updated, nil)
		require.NoError(t, err)

		got, _, err := acl.RoleReadByName("app-role-renamed", nil)
		require.NoError(t, err)
		require.Equal(t, role.ID, got.ID)
		role = got
	})

	t.Run("invalid roles are rejected", func(t *testing.T) {
		for name, tc := range map[string]struct {
			role    *api.ACLRole
			wantErr string
		}{
			"a duplicate name":               {&api.ACLRole{Name: "app-role-renamed"}, "already exists"},
			"a policy that does not exist":   {&api.ACLRole{Name: "ghost-policy", Policies: []*api.ACLRolePolicyLink{{Name: "ghost"}}}, "No such ACL policy"},
			"an invalid service identity":    {&api.ACLRole{Name: "bad-identity", ServiceIdentities: []*api.ACLServiceIdentity{{ServiceName: "bad name"}}}, "invalid name"},
			"a node identity without its dc": {&api.ACLRole{Name: "bad-node", NodeIdentities: []*api.ACLNodeIdentity{{NodeName: "n1"}}}, "datacenter"},
		} {
			_, _, err := acl.RoleCreate(tc.role, nil)
			require.Error(t, err, name)
			require.Contains(t, err.Error(), tc.wantErr, name)
		}
	})

	t.Run("deleting a policy detaches it from roles", func(t *testing.T) {
		policy, _, err := acl.PolicyReadByName("role-policy", nil)
		require.NoError(t, err)
		_, err = acl.PolicyDelete(policy.ID, nil)
		require.NoError(t, err)

		got, _, err := acl.RoleRead(role.ID, nil)
		require.NoError(t, err)
		require.Empty(t, got.Policies, "the link to the deleted policy is gone")
		require.Len(t, got.ServiceIdentities, 1, "the rest of the role is intact")
	})

	t.Run("deleting a role removes it", func(t *testing.T) {
		_, err := acl.RoleDelete(role.ID, nil)
		require.NoError(t, err)

		got, _, err := acl.RoleRead(role.ID, nil)
		require.NoError(t, err)
		require.Nil(t, got)
	})
}

// TestACL_TokenLifecycle covers token creation, inspection, updates and the
// rules for secrets, accessors and expiration.
func TestACL_TokenLifecycle(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "", "")
	acl := e.admin.ACL()

	_, _, err := acl.PolicyCreate(&api.ACLPolicy{Name: "token-policy", Rules: `service_prefix "" { policy = "read" }`}, nil)
	require.NoError(t, err)
	_, _, err = acl.RoleCreate(&api.ACLRole{Name: "token-role", Policies: []*api.ACLRolePolicyLink{{Name: "token-policy"}}}, nil)
	require.NoError(t, err)

	t.Run("a token carries policies, roles and identities and can identify itself", func(t *testing.T) {
		token, _, err := acl.TokenCreate(&api.ACLToken{
			Description:       "full token",
			Policies:          []*api.ACLTokenPolicyLink{{Name: "token-policy"}},
			Roles:             []*api.ACLTokenRoleLink{{Name: "token-role"}},
			ServiceIdentities: []*api.ACLServiceIdentity{{ServiceName: "web"}},
			NodeIdentities:    []*api.ACLNodeIdentity{{NodeName: "node-a", Datacenter: "dc1"}},
		}, nil)
		require.NoError(t, err)
		require.NotEmpty(t, token.AccessorID)
		require.NotEmpty(t, token.SecretID)
		require.NotEqual(t, token.AccessorID, token.SecretID)

		self, _, err := e.tokenClient(t, token.SecretID).ACL().TokenReadSelf(nil)
		require.NoError(t, err)
		require.Equal(t, token.AccessorID, self.AccessorID)
		require.Equal(t, "token-policy", self.Policies[0].Name)
		require.Equal(t, "token-role", self.Roles[0].Name)
		require.Equal(t, "web", self.ServiceIdentities[0].ServiceName)
		require.Equal(t, "node-a", self.NodeIdentities[0].NodeName)
	})

	t.Run("an accessor and secret can be chosen, but not reused", func(t *testing.T) {
		const accessor = "11111111-1111-1111-1111-111111111111"
		const secret = "22222222-2222-2222-2222-222222222222"

		created, _, err := acl.TokenCreate(&api.ACLToken{AccessorID: accessor, SecretID: secret}, nil)
		require.NoError(t, err)
		require.Equal(t, secret, created.SecretID)

		_, _, err = acl.TokenCreate(&api.ACLToken{AccessorID: accessor}, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "AccessorID is already in use")
	})

	t.Run("a token cannot reference a role or policy that does not exist", func(t *testing.T) {
		_, _, err := acl.TokenCreate(&api.ACLToken{Roles: []*api.ACLTokenRoleLink{{Name: "ghost"}}}, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "No such ACL role")

		_, _, err = acl.TokenCreate(&api.ACLToken{Policies: []*api.ACLTokenPolicyLink{{Name: "ghost"}}}, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "No such ACL policy")
	})

	t.Run("updates change the description and links but never the secret", func(t *testing.T) {
		token, _, err := acl.TokenCreate(&api.ACLToken{Description: "original"}, nil)
		require.NoError(t, err)

		token.Description = "changed"
		token.Policies = []*api.ACLTokenPolicyLink{{Name: "token-policy"}}
		updated, _, err := acl.TokenUpdate(token, nil)
		require.NoError(t, err)
		require.Equal(t, "changed", updated.Description)
		require.Len(t, updated.Policies, 1)

		token.SecretID = "33333333-3333-3333-3333-333333333333"
		_, _, err = acl.TokenUpdate(token, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "SecretID is not permitted")
	})

	t.Run("a token can be cloned", func(t *testing.T) {
		source, _, err := acl.TokenCreate(&api.ACLToken{Description: "source", Policies: []*api.ACLTokenPolicyLink{{Name: "token-policy"}}}, nil)
		require.NoError(t, err)

		clone, _, err := acl.TokenClone(source.AccessorID, "the copy", nil)
		require.NoError(t, err)
		require.NotEqual(t, source.AccessorID, clone.AccessorID)
		require.NotEqual(t, source.SecretID, clone.SecretID, "a clone has its own secret")
		require.Equal(t, "the copy", clone.Description)
		require.Equal(t, "token-policy", clone.Policies[0].Name, "and the same permissions")
	})

	t.Run("local tokens are marked as such", func(t *testing.T) {
		token, _, err := acl.TokenCreate(&api.ACLToken{Description: "dc-local", Local: true}, nil)
		require.NoError(t, err)
		require.True(t, token.Local)
	})

	t.Run("expiration must be at least a minute away and unambiguous", func(t *testing.T) {
		_, _, err := acl.TokenCreate(&api.ACLToken{ExpirationTTL: 10 * time.Second}, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot be less than 1m0s in the future")

		past := time.Now().Add(-time.Hour)
		_, _, err = acl.TokenCreate(&api.ACLToken{ExpirationTime: &past}, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot be before CreateTime")

		future := time.Now().Add(time.Hour)
		_, _, err = acl.TokenCreate(&api.ACLToken{ExpirationTTL: 2 * time.Minute, ExpirationTime: &future}, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot both be set")

		token, _, err := acl.TokenCreate(&api.ACLToken{ExpirationTTL: 5 * time.Minute}, nil)
		require.NoError(t, err)
		require.NotNil(t, token.ExpirationTime)
		require.WithinDuration(t, time.Now().Add(5*time.Minute), *token.ExpirationTime, time.Minute)
	})

	t.Run("deleting a token removes it, and deleting it again fails", func(t *testing.T) {
		token, _, err := acl.TokenCreate(&api.ACLToken{Description: "short lived"}, nil)
		require.NoError(t, err)
		require.Contains(t, tokenAccessors(t, e.admin), token.AccessorID)

		_, err = acl.TokenDelete(token.AccessorID, nil)
		require.NoError(t, err)
		require.NotContains(t, tokenAccessors(t, e.admin), token.AccessorID)

		_, err = acl.TokenDelete(token.AccessorID, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Cannot find token to delete")
	})

	t.Run("a revoked token stops working", func(t *testing.T) {
		token, _, err := acl.TokenCreate(&api.ACLToken{Policies: []*api.ACLTokenPolicyLink{{Name: "token-policy"}}}, nil)
		require.NoError(t, err)
		c := e.tokenClient(t, token.SecretID)

		_, _, err = c.Catalog().Services(nil)
		require.NoError(t, err)

		_, err = acl.TokenDelete(token.AccessorID, nil)
		require.NoError(t, err)

		eventually(t, func(r *retry.R) {
			_, _, err := c.Catalog().Services(nil)
			require.Error(r, err)
			require.Contains(r, err.Error(), "ACL not found")
		})
	})
}
