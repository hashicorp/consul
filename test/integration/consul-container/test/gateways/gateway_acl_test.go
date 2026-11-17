// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package gateways

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
)

// newToken creates a token holding a single policy with the given rules and
// returns an API client that authenticates with it.
func (e *apiEnv) newToken(t *testing.T, name, rules string) *api.Client {
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

// TestGateways_ACLs verifies who may configure gateways, register gateway
// instances, and see which services a gateway fronts, with default-deny ACLs.
func TestGateways_ACLs(t *testing.T) {
	t.Parallel()

	e := newAPIEnv(t, true)

	for _, svc := range []string{"web", "secret"} {
		setEntry(t, e.server, &api.ServiceConfigEntry{Kind: api.ServiceDefaults, Name: svc, Protocol: "tcp"})
	}

	meshAdmin := e.newToken(t, "mesh-admin", `mesh = "write"`)
	gatewayOperator := e.newToken(t, "gateway-operator", `
service "ingress" { policy = "write" }
service_prefix "" { policy = "read" }
node_prefix "" { policy = "read" }`)
	reader := e.newToken(t, "reader", `
service_prefix "" { policy = "read" }
node_prefix "" { policy = "read" }`)
	limitedReader := e.newToken(t, "limited-reader", `
service "ingress" { policy = "read" }
service "web" { policy = "read" }`)
	unrelated := e.newToken(t, "unrelated", `service "something-else" { policy = "read" }`)

	ingress := &api.IngressGatewayConfigEntry{
		Kind: api.IngressGateway,
		Name: "ingress",
		Listeners: []api.IngressListener{
			{Port: 8080, Protocol: "tcp", Services: []api.IngressService{{Name: "web"}}},
			{Port: 8081, Protocol: "tcp", Services: []api.IngressService{{Name: "secret"}}},
		},
	}
	terminating := &api.TerminatingGatewayConfigEntry{
		Kind:     api.TerminatingGateway,
		Name:     "terminating",
		Services: []api.LinkedService{{Name: "legacy-db"}},
	}

	t.Run("writing gateway config entries requires mesh:write", func(t *testing.T) {
		for name, c := range map[string]*api.Client{
			"gateway operator": gatewayOperator,
			"reader":           reader,
			"unrelated":        unrelated,
		} {
			for _, entry := range []api.ConfigEntry{ingress, terminating} {
				ok, _, err := c.ConfigEntries().Set(entry, nil)
				require.Error(t, err, "%s must not write %s", name, entry.GetKind())
				require.False(t, ok)
				require.Contains(t, err.Error(), "lacks permission 'mesh:write'", name)
			}
		}

		for _, entry := range []api.ConfigEntry{ingress, terminating} {
			ok, _, err := meshAdmin.ConfigEntries().Set(entry, nil)
			require.NoError(t, err, entry.GetKind())
			require.True(t, ok)
		}
	})

	t.Run("reading an entry requires service:read on the gateway", func(t *testing.T) {
		for name, c := range map[string]*api.Client{
			"gateway operator": gatewayOperator,
			"reader":           reader,
			"limited reader":   limitedReader,
		} {
			entry, _, err := c.ConfigEntries().Get(api.IngressGateway, "ingress", nil)
			require.NoError(t, err, name)
			require.Equal(t, "ingress", entry.GetName())
		}

		for name, c := range map[string]*api.Client{
			"mesh admin": meshAdmin, // mesh:write does not grant service:read
			"unrelated":  unrelated,
		} {
			_, _, err := c.ConfigEntries().Get(api.IngressGateway, "ingress", nil)
			require.Error(t, err, name)
			require.Contains(t, err.Error(), "403", name)
		}
	})

	t.Run("the gateway-services view only lists services the token can read", func(t *testing.T) {
		retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
			require.Equal(r, []string{"secret", "web"}, sortedKeys(linkedServices(r, reader, "ingress")))
			require.Equal(r, []string{"web"}, sortedKeys(linkedServices(r, limitedReader, "ingress")))
		})

		_, _, err := unrelated.Catalog().GatewayServices("ingress", nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "lacks permission 'service:read' on \"ingress\"")

		_, _, err = meshAdmin.Catalog().GatewayServices("ingress", nil)
		require.Error(t, err, "mesh:write is not enough to read the gateway's services")
	})

	t.Run("registering a gateway instance requires service:write on the gateway", func(t *testing.T) {
		reg := &api.AgentServiceRegistration{
			Kind:    api.ServiceKindIngressGateway,
			ID:      "ingress-1",
			Name:    "ingress",
			Address: "10.9.9.9",
			Port:    8080,
		}

		for name, c := range map[string]*api.Client{
			"reader":     reader,
			"unrelated":  unrelated,
			"mesh admin": meshAdmin,
		} {
			err := c.Agent().ServiceRegister(reg)
			require.Error(t, err, name)
			require.Contains(t, err.Error(), "lacks permission 'service:write' on \"ingress\"", name)
		}

		require.NoError(t, gatewayOperator.Agent().ServiceRegister(reg))

		retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
			entries, _, err := reader.Health().Ingress("web", false, nil)
			require.NoError(r, err)
			require.Len(r, entries, 1)
		})
	})

	t.Run("health by ingress is denied without access to the exposed service", func(t *testing.T) {
		_, _, err := unrelated.Health().Ingress("web", false, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "403")
	})
}
