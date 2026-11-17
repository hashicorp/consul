// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package acl

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

func registerService(c *api.Client, name string) error {
	return c.Agent().ServiceRegister(&api.AgentServiceRegistration{Name: name, Port: 9000})
}

func registerNode(c *api.Client, node string) error {
	_, err := c.Catalog().Register(&api.CatalogRegistration{Node: node, Address: "10.0.0.1"}, nil)
	return err
}

// TestACL_Identities verifies service and node identities: shorthand for the
// permissions a service or a node needs, without writing a policy.
func TestACL_Identities(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "", "")

	t.Run("a service identity allows registering that service only", func(t *testing.T) {
		token, _, err := e.admin.ACL().TokenCreate(&api.ACLToken{
			Description:       "svc-id",
			ServiceIdentities: []*api.ACLServiceIdentity{{ServiceName: "payments"}},
		}, nil)
		require.NoError(t, err)
		c := e.tokenClient(t, token.SecretID)

		require.NoError(t, registerService(c, "payments"))
		requireDenied(t, registerService(c, "billing"), "another service")
	})

	t.Run("a node identity allows registering that node only", func(t *testing.T) {
		token, _, err := e.admin.ACL().TokenCreate(&api.ACLToken{
			Description:    "node-id",
			NodeIdentities: []*api.ACLNodeIdentity{{NodeName: "edge-node", Datacenter: "dc1"}},
		}, nil)
		require.NoError(t, err)
		c := e.tokenClient(t, token.SecretID)

		require.NoError(t, registerNode(c, "edge-node"))
		requireDenied(t, registerNode(c, "other-node"), "another node")
	})

	t.Run("identities may be attached to a role and shared", func(t *testing.T) {
		_, _, err := e.admin.ACL().RoleCreate(&api.ACLRole{
			Name:              "shared-role",
			ServiceIdentities: []*api.ACLServiceIdentity{{ServiceName: "shared-svc"}},
		}, nil)
		require.NoError(t, err)

		for i := 0; i < 2; i++ {
			token, _, err := e.admin.ACL().TokenCreate(&api.ACLToken{Roles: []*api.ACLTokenRoleLink{{Name: "shared-role"}}}, nil)
			require.NoError(t, err)
			require.NoError(t, registerService(e.tokenClient(t, token.SecretID), "shared-svc"))
		}
	})
}

// TestACL_TemplatedPolicies verifies the built-in templated policies: named,
// parameterized rule sets that replace hand-written policies for common roles.
func TestACL_TemplatedPolicies(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "", "")
	acl := e.admin.ACL()

	t.Run("the built-in templates are listed", func(t *testing.T) {
		templates, _, err := acl.TemplatedPolicyList(nil)
		require.NoError(t, err)

		var names []string
		for name := range templates {
			names = append(names, name)
		}
		require.ElementsMatch(t, []string{
			"builtin/api-gateway", "builtin/dns", "builtin/node", "builtin/nomad-client", "builtin/nomad-server", "builtin/service",
		}, names)
	})

	t.Run("a template describes its variables", func(t *testing.T) {
		tpl, _, err := acl.TemplatedPolicyReadByName("builtin/service", nil)
		require.NoError(t, err)
		require.Equal(t, "builtin/service", tpl.TemplateName)
		require.Contains(t, tpl.Schema, "name", "the service template takes the service name")
	})

	t.Run("previewing shows the rules a template produces", func(t *testing.T) {
		preview, _, err := acl.TemplatedPolicyPreview(&api.ACLTemplatedPolicy{
			TemplateName:      "builtin/service",
			TemplateVariables: &api.ACLTemplatedPolicyVariables{Name: "web"},
		}, nil)
		require.NoError(t, err)

		require.Contains(t, preview.Rules, `service "web"`)
		require.Contains(t, preview.Rules, `service "web-sidecar-proxy"`, "the sidecar is covered too")
		require.Contains(t, preview.Description, "builtin/service")
	})

	t.Run("a token from the service template can run that service and its sidecar", func(t *testing.T) {
		token, _, err := acl.TokenCreate(&api.ACLToken{
			Description: "from template",
			TemplatedPolicies: []*api.ACLTemplatedPolicy{{
				TemplateName:      "builtin/service",
				TemplateVariables: &api.ACLTemplatedPolicyVariables{Name: "orders"},
			}},
		}, nil)
		require.NoError(t, err)
		require.Equal(t, "builtin/service", token.TemplatedPolicies[0].TemplateName)
		c := e.tokenClient(t, token.SecretID)

		require.NoError(t, registerService(c, "orders"))
		require.NoError(t, registerService(c, "orders-sidecar-proxy"))
		requireDenied(t, registerService(c, "inventory"), "a service the template does not cover")
	})

	t.Run("a token from the dns template can discover services but not change anything", func(t *testing.T) {
		require.NoError(t, registerService(e.adminClient, "discoverable"))
		token, _, err := acl.TokenCreate(&api.ACLToken{
			TemplatedPolicies: []*api.ACLTemplatedPolicy{{TemplateName: "builtin/dns"}},
		}, nil)
		require.NoError(t, err)
		c := e.tokenClient(t, token.SecretID)

		eventually(t, func(r *retry.R) {
			instances, _, err := c.Catalog().Service("discoverable", "", nil)
			require.NoError(r, err)
			require.Len(r, instances, 1)
		})
		requireDenied(t, registerService(c, "anything"), "a read-only template")
	})

	t.Run("invalid template use is rejected", func(t *testing.T) {
		_, _, err := acl.TokenCreate(&api.ACLToken{
			TemplatedPolicies: []*api.ACLTemplatedPolicy{{TemplateName: "builtin/does-not-exist"}},
		}, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "no such ACL templated policy")

		_, _, err = acl.TokenCreate(&api.ACLToken{
			TemplatedPolicies: []*api.ACLTemplatedPolicy{{TemplateName: "builtin/service"}},
		}, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "validation error")
	})
}
