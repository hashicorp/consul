// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package ops

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

func usage(t retry.TestingTB, n node) api.ServiceUsage {
	u, _, err := n.Client().Operator().Usage(nil)
	require.NoError(t, err)
	return u.Usage["dc1"]
}

// TestOperatorUsage verifies the service and node counts reported by
// /v1/operator/usage and `consul operator usage instances`.
func TestOperatorUsage(t *testing.T) {
	e := newEnv(t, "", "")
	srv := e.server
	catalog := srv.Client().Catalog()

	// The server and the client agent register themselves at their own pace.
	var baseline api.ServiceUsage
	eventually(t, func(r *retry.R) {
		baseline = usage(r, srv)
		require.Equal(r, 2, baseline.Nodes)
	})

	register := func(node, address string, svc *api.AgentService) {
		_, err := catalog.Register(&api.CatalogRegistration{Node: node, Address: address, Service: svc}, nil)
		require.NoError(t, err)
	}
	register("usage-a", "192.0.2.1", &api.AgentService{ID: "web-1", Service: "web", Port: 80})
	register("usage-b", "192.0.2.2", &api.AgentService{ID: "web-2", Service: "web", Port: 80})
	register("usage-a", "192.0.2.1", &api.AgentService{
		ID: "web-proxy", Service: "web-sidecar-proxy", Kind: api.ServiceKindConnectProxy, Port: 21000,
		Proxy: &api.AgentServiceConnectProxyConfig{DestinationServiceName: "web"},
	})
	register("usage-a", "192.0.2.1", &api.AgentService{ID: "mgw", Service: "mesh-gateway", Kind: api.ServiceKindMeshGateway, Port: 8443})
	register("usage-b", "192.0.2.2", &api.AgentService{ID: "native", Service: "native", Port: 9000, Connect: &api.AgentServiceConnect{Native: true}})

	t.Run("counts follow registrations", func(t *testing.T) {
		eventually(t, func(r *retry.R) {
			got := usage(r, srv)
			require.Equal(r, baseline.Nodes+2, got.Nodes)
			require.Equal(r, baseline.Services+4, got.Services)
			require.Equal(r, baseline.ServiceInstances+5, got.ServiceInstances)
			require.Equal(r, baseline.ConnectServiceInstances[string(api.ServiceKindConnectProxy)]+1, got.ConnectServiceInstances[string(api.ServiceKindConnectProxy)])
			require.Equal(r, baseline.ConnectServiceInstances[string(api.ServiceKindMeshGateway)]+1, got.ConnectServiceInstances[string(api.ServiceKindMeshGateway)])
			require.Equal(r, baseline.ConnectServiceInstances["connect-native"]+1, got.ConnectServiceInstances["connect-native"])
		})
	})

	t.Run("only application instances are billable", func(t *testing.T) {
		got := usage(t, srv)
		// Two web instances and the Connect native service. The sidecar proxy,
		// the gateway and Consul's own service are not billable.
		require.Equal(t, baseline.BillableServiceInstances+3, got.BillableServiceInstances)
	})

	t.Run("the CLI reports the same numbers", func(t *testing.T) {
		want := usage(t, srv)

		out := requireCLI(t, srv, "operator", "usage", "instances")
		require.Contains(t, out, fmt.Sprintf("Billable Service Instances Total: %d", want.BillableServiceInstances))
		require.Contains(t, out, fmt.Sprintf("dc1 Billable Service Instances: %d", want.BillableServiceInstances))
	})

	t.Run("deregistering brings the counts back", func(t *testing.T) {
		for _, n := range []string{"usage-a", "usage-b"} {
			_, err := catalog.Deregister(&api.CatalogDeregistration{Node: n}, nil)
			require.NoError(t, err)
		}

		eventually(t, func(r *retry.R) {
			require.Equal(r, baseline, usage(r, srv))
		})
	})
}

func requireCLI(t *testing.T, n node, args ...string) string {
	t.Helper()
	out, code := n.CLI(t, args...)
	require.Equal(t, 0, code, out)
	return out
}
