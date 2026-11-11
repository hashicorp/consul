// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package connect

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
)

// TestConfigEntries_PropagateToProxyRegistration verifies that proxy-defaults
// and service-defaults are merged into the sidecar's effective configuration
// as served by the client agent, which is what xDS is generated from, and that
// service-defaults take precedence over proxy-defaults.
func TestConfigEntries_PropagateToProxyRegistration(t *testing.T) {
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
	const sidecarID = "web-sidecar-proxy"

	effective := func(r retry.TestingTB) *api.AgentService {
		svc, _, err := e.client.Agent().Service(sidecarID, nil)
		require.NoError(r, err)
		return svc
	}
	waitFor := func(t *testing.T, assert func(r *retry.R, svc *api.AgentService)) {
		t.Helper()
		retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) { assert(r, effective(r)) })
	}

	waitFor(t, func(r *retry.R, svc *api.AgentService) {
		require.NotContains(r, svc.Proxy.Config, "protocol")
		require.Empty(r, svc.Proxy.MeshGateway.Mode)
	})

	t.Run("proxy-defaults apply to the proxy and its upstreams", func(t *testing.T) {
		setEntry(t, e.server, &api.ProxyConfigEntry{
			Kind:        api.ProxyDefaults,
			Name:        api.ProxyConfigGlobal,
			Config:      map[string]any{"protocol": "http"},
			MeshGateway: api.MeshGatewayConfig{Mode: api.MeshGatewayModeLocal},
		})

		waitFor(t, func(r *retry.R, svc *api.AgentService) {
			require.Equal(r, "http", svc.Proxy.Config["protocol"])
			require.Equal(r, api.MeshGatewayModeLocal, svc.Proxy.MeshGateway.Mode)
			require.Len(r, svc.Proxy.Upstreams, 1)
			require.Equal(r, "http", svc.Proxy.Upstreams[0].Config["protocol"])
			require.Equal(r, api.MeshGatewayModeLocal, svc.Proxy.Upstreams[0].MeshGateway.Mode)
		})
	})

	t.Run("service-defaults override proxy-defaults and configure upstreams", func(t *testing.T) {
		setEntry(t, e.server, &api.ServiceConfigEntry{
			Kind:     api.ServiceDefaults,
			Name:     "web",
			Protocol: "grpc",
			UpstreamConfig: &api.UpstreamConfiguration{
				Defaults: &api.UpstreamConfig{ConnectTimeoutMs: 1234},
				Overrides: []*api.UpstreamConfig{{
					Name:               "db",
					PassiveHealthCheck: &api.PassiveHealthCheck{MaxFailures: 3},
				}},
			},
		})

		waitFor(t, func(r *retry.R, svc *api.AgentService) {
			require.Equal(r, "grpc", svc.Proxy.Config["protocol"], "service-defaults win over proxy-defaults")

			require.Len(r, svc.Proxy.Upstreams, 1)
			up := svc.Proxy.Upstreams[0].Config
			require.EqualValues(r, 1234, up["connect_timeout_ms"])
			raw, err := json.Marshal(up["passive_health_check"])
			require.NoError(r, err)
			require.Contains(r, string(raw), `"MaxFailures":3`)
		})
	})

	t.Run("removing service-defaults falls back to proxy-defaults", func(t *testing.T) {
		_, err := e.server.ConfigEntries().Delete(api.ServiceDefaults, "web", nil)
		require.NoError(t, err)

		waitFor(t, func(r *retry.R, svc *api.AgentService) {
			require.Equal(r, "http", svc.Proxy.Config["protocol"])
			require.NotContains(r, svc.Proxy.Upstreams[0].Config, "connect_timeout_ms")
		})
	})

	t.Run("removing proxy-defaults clears the configuration", func(t *testing.T) {
		_, err := e.server.ConfigEntries().Delete(api.ProxyDefaults, api.ProxyConfigGlobal, nil)
		require.NoError(t, err)

		waitFor(t, func(r *retry.R, svc *api.AgentService) {
			require.NotContains(r, svc.Proxy.Config, "protocol")
			require.Empty(r, svc.Proxy.MeshGateway.Mode)
		})
	})
}
