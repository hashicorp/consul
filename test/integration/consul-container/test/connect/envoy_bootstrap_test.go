// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package connect

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
)

const grpcAddr = "127.0.0.1:8502"

// TestConnectEnvoy_Bootstrap exercises `consul connect envoy -bootstrap`, which
// renders the Envoy bootstrap configuration for a registered proxy without
// starting Envoy. It verifies the flags that decide how Envoy identifies
// itself and reaches Consul.
func TestConnectEnvoy_Bootstrap(t *testing.T) {
	t.Parallel()

	e := newEnv(t)

	require.NoError(t, e.client.Agent().ServiceRegister(&api.AgentServiceRegistration{
		Name: "web",
		Port: 8080,
		Connect: &api.AgentServiceConnect{
			SidecarService: &api.AgentServiceRegistration{
				Proxy: &api.AgentServiceConnectProxyConfig{
					Upstreams: []api.Upstream{{DestinationName: "db", LocalBindPort: 9191}},
				},
			},
		},
	}))
	require.NoError(t, e.client.Agent().ServiceRegister(&api.AgentServiceRegistration{
		Name: "metrics",
		Port: 8081,
		Connect: &api.AgentServiceConnect{
			SidecarService: &api.AgentServiceRegistration{
				Proxy: &api.AgentServiceConnectProxyConfig{
					Config: map[string]any{"envoy_prometheus_bind_addr": "0.0.0.0:20200"},
				},
			},
		},
	}))

	bootstrap := func(t *testing.T, args ...string) map[string]any {
		t.Helper()
		var out map[string]any
		retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
			res := e.cliStdout(t, append([]string{"connect", "envoy", "-bootstrap", "-grpc-addr", grpcAddr}, args...)...)
			require.Equal(r, 0, res.ExitCode, res.Output)
			out = decodeJSON(t, res.Output)
		})
		return out
	}

	t.Run("sidecar-for identifies the sidecar proxy", func(t *testing.T) {
		b := bootstrap(t, "-sidecar-for", "web")

		require.Equal(t, "web-sidecar-proxy", dig(t, b, "node", "id"))
		require.Equal(t, "web", dig(t, b, "node", "cluster"))
		require.Equal(t, "default", dig(t, b, "node", "metadata", "namespace"))
		require.Equal(t, "default", dig(t, b, "node", "metadata", "partition"))
	})

	t.Run("proxy-id is equivalent to sidecar-for", func(t *testing.T) {
		b := bootstrap(t, "-proxy-id", "web-sidecar-proxy")

		require.Equal(t, "web-sidecar-proxy", dig(t, b, "node", "id"))
		require.Equal(t, "web", dig(t, b, "node", "cluster"))
	})

	t.Run("xDS is delta ADS over the local agent gRPC address", func(t *testing.T) {
		b := bootstrap(t, "-sidecar-for", "web")

		require.Equal(t, "DELTA_GRPC", dig(t, b, "dynamic_resources", "ads_config", "api_type"))
		require.Equal(t, "local_agent", dig(t, b, "dynamic_resources", "ads_config", "grpc_services", "envoy_grpc", "cluster_name"))

		require.Contains(t, clusterNames(t, b), "local_agent")
		endpoint := dig(t, b, "static_resources", "clusters", 0, "loadAssignment", "endpoints", 0, "lbEndpoints", 0, "endpoint", "address", "socket_address")
		require.Equal(t, "127.0.0.1", dig(t, endpoint, "address"))
		require.EqualValues(t, 8502, dig(t, endpoint, "port_value"))
	})

	t.Run("token is sent as xDS request metadata", func(t *testing.T) {
		b := bootstrap(t, "-sidecar-for", "web", "-token", "secret-token")

		md := dig(t, b, "dynamic_resources", "ads_config", "grpc_services", "initial_metadata", 0)
		require.Equal(t, "x-consul-token", dig(t, md, "key"))
		require.Equal(t, "secret-token", dig(t, md, "value"))
	})

	t.Run("admin-bind sets the Envoy admin address", func(t *testing.T) {
		b := bootstrap(t, "-sidecar-for", "web", "-admin-bind", "127.0.0.1:19005")

		addr := dig(t, b, "admin", "address", "socket_address")
		require.Equal(t, "127.0.0.1", dig(t, addr, "address"))
		require.EqualValues(t, 19005, dig(t, addr, "port_value"))
	})

	t.Run("ready listener", func(t *testing.T) {
		b := bootstrap(t, "-sidecar-for", "web", "-envoy-ready-bind-address", "0.0.0.0", "-envoy-ready-bind-port", "21000")

		require.Contains(t, listenerNames(b), "envoy_ready_listener")
	})

	t.Run("prometheus listener and backend", func(t *testing.T) {
		plain := bootstrap(t, "-sidecar-for", "metrics")
		require.NotContains(t, clusterNames(t, plain), "prometheus_backend")

		b := bootstrap(t, "-sidecar-for", "metrics", "-prometheus-backend-port", "9102")
		require.Contains(t, clusterNames(t, b), "prometheus_backend")
		require.Contains(t, listenerNames(b), "envoy_prometheus_metrics_listener")
	})

	t.Run("unknown sidecar is rejected", func(t *testing.T) {
		res := e.cliCombined(t, "connect", "envoy", "-bootstrap", "-grpc-addr", grpcAddr, "-sidecar-for", "does-not-exist")

		require.NotEqual(t, 0, res.ExitCode)
		require.Contains(t, res.Output, "No sidecar proxy registered for does-not-exist")
	})

	t.Run("missing proxy identity is rejected", func(t *testing.T) {
		res := e.cliCombined(t, "connect", "envoy", "-bootstrap", "-grpc-addr", grpcAddr)

		require.NotEqual(t, 0, res.ExitCode)
	})
}

// TestConnectEnvoy_GatewayRegister verifies that `-gateway=mesh -register`
// creates the gateway service in the catalog with the requested addresses and
// emits a bootstrap whose node identity is that gateway.
func TestConnectEnvoy_GatewayRegister(t *testing.T) {
	t.Parallel()

	e := newEnv(t)

	var bootstrap map[string]any
	retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
		res := e.cliStdout(t, "connect", "envoy",
			"-bootstrap", "-grpc-addr", grpcAddr,
			"-gateway", "mesh", "-register", "-service", "mgw",
			"-address", "127.0.0.1:8443",
		)
		require.Equal(r, 0, res.ExitCode, res.Output)
		bootstrap = decodeJSON(t, res.Output)
	})

	require.Equal(t, "mgw", dig(t, bootstrap, "node", "id"))
	require.Equal(t, "mgw", dig(t, bootstrap, "node", "cluster"))

	retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
		entries, _, err := e.server.Health().Service("mgw", "", false, nil)
		require.NoError(r, err)
		require.Len(r, entries, 1)
		require.Equal(r, api.ServiceKindMeshGateway, entries[0].Service.Kind)
		require.Equal(r, "127.0.0.1", entries[0].Service.Address)
		require.Equal(r, 8443, entries[0].Service.Port)
	})
}
