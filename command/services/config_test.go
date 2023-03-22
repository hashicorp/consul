package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
)

// This test ensures that dev mode doesn't register services by default.
// We depend on this behavior for ServiesFromFiles so we want to fail
// tests if that ever changes.
func TestDevModeHasNoServices(t *testing.T) {
	devMode := true
	opts := config.LoadOpts{
		DevMode: &devMode,
		HCL:     []string{`node_name = "dummy"`},
	}
	result, err := config.Load(opts)
	require.NoError(t, err)
	require.Len(t, result.Warnings, 0)
	require.Len(t, result.RuntimeConfig.Services, 0)
}

func TestInvalidNodeNameWarning(t *testing.T) {
	devMode := true
	opts := config.LoadOpts{
		DevMode: &devMode,
		HCL:     []string{`node_name = "dummy.local"`},
	}
	result, err := config.Load(opts)
	require.NoError(t, err)
	require.Len(t, result.Warnings, 1)
	require.Contains(t, result.Warnings[0], "will not be discoverable via DNS due to invalid characters. Valid characters include all alpha-numerics and dashes.")
}

func TestStructsToAgentService(t *testing.T) {
	t.Parallel()

	cases := []struct {
		Name   string
		Input  *structs.ServiceDefinition
		Output *api.AgentServiceRegistration
	}{
		{
			"Basic service with port",
			&structs.ServiceDefinition{
				Name: "web",
				Tags: []string{"leader"},
				Port: 1234,
			},
			&api.AgentServiceRegistration{
				Name: "web",
				Tags: []string{"leader"},
				Port: 1234,
			},
		},
		{
			"Service with a check",
			&structs.ServiceDefinition{
				Name: "web",
				Check: structs.CheckType{
					Name: "ping",
				},
			},
			&api.AgentServiceRegistration{
				Name: "web",
				Check: &api.AgentServiceCheck{
					Name: "ping",
				},
			},
		},
		{
			"Service with an unnamed check",
			&structs.ServiceDefinition{
				Name: "web",
				Check: structs.CheckType{
					TTL: 5 * time.Second,
				},
			},
			&api.AgentServiceRegistration{
				Name: "web",
				Check: &api.AgentServiceCheck{
					TTL: "5s",
				},
			},
		},
		{
			"Service with a zero-value check",
			&structs.ServiceDefinition{
				Name:  "web",
				Check: structs.CheckType{},
			},
			&api.AgentServiceRegistration{
				Name:  "web",
				Check: nil,
			},
		},
		{
			"Service with checks",
			&structs.ServiceDefinition{
				Name: "web",
				Checks: structs.CheckTypes{
					&structs.CheckType{
						Name: "ping",
					},
					&structs.CheckType{
						Name: "pong",
					},
				},
			},
			&api.AgentServiceRegistration{
				Name: "web",
				Checks: api.AgentServiceChecks{
					&api.AgentServiceCheck{
						Name: "ping",
					},
					&api.AgentServiceCheck{
						Name: "pong",
					},
				},
			},
		},
		{
			"Proxy service",
			&structs.ServiceDefinition{
				Name: "web-proxy",
				Kind: structs.ServiceKindConnectProxy,
				Tags: []string{"leader"},
				Port: 1234,
				Proxy: &structs.ConnectProxyConfig{
					DestinationServiceID:   "web1",
					DestinationServiceName: "web",
					LocalServiceAddress:    "127.0.0.1",
					LocalServicePort:       8181,
					Upstreams:              structs.TestUpstreams(t, false),
					Mode:                   structs.ProxyModeTransparent,
					Config: map[string]interface{}{
						"foo": "bar",
					},
				},
			},
			&api.AgentServiceRegistration{
				Name: "web-proxy",
				Tags: []string{"leader"},
				Port: 1234,
				Kind: api.ServiceKindConnectProxy,
				Proxy: &api.AgentServiceConnectProxyConfig{
					DestinationServiceID:   "web1",
					DestinationServiceName: "web",
					LocalServiceAddress:    "127.0.0.1",
					LocalServicePort:       8181,
					Upstreams:              structs.TestUpstreams(t, false).ToAPI(),
					Mode:                   api.ProxyModeTransparent,
					Config: map[string]interface{}{
						"foo": "bar",
					},
				},
			},
		},
		{
			"TProxy proxy service w/ access logs",
			&structs.ServiceDefinition{
				Name: "web-proxy",
				Kind: structs.ServiceKindConnectProxy,
				Tags: []string{"leader"},
				Port: 1234,
				Proxy: &structs.ConnectProxyConfig{
					DestinationServiceID:   "web1",
					DestinationServiceName: "web",
					LocalServiceAddress:    "127.0.0.1",
					LocalServicePort:       8181,
					Upstreams:              structs.TestUpstreams(t, false),
					Mode:                   structs.ProxyModeTransparent,
					TransparentProxy: structs.TransparentProxyConfig{
						OutboundListenerPort: 808,
					},
					AccessLogs: structs.AccessLogsConfig{
						Enabled:             true,
						DisableListenerLogs: true,
						Type:                structs.FileLogSinkType,
						Path:                "/var/logs/envoy.logs",
						TextFormat:          "MY START TIME %START_TIME%",
					},
					Config: map[string]interface{}{
						"foo": "bar",
					},
				},
			},
			&api.AgentServiceRegistration{
				Name: "web-proxy",
				Tags: []string{"leader"},
				Port: 1234,
				Kind: api.ServiceKindConnectProxy,
				Proxy: &api.AgentServiceConnectProxyConfig{
					DestinationServiceID:   "web1",
					DestinationServiceName: "web",
					LocalServiceAddress:    "127.0.0.1",
					LocalServicePort:       8181,
					Upstreams:              structs.TestUpstreams(t, false).ToAPI(),
					Mode:                   api.ProxyModeTransparent,
					TransparentProxy: &api.TransparentProxyConfig{
						OutboundListenerPort: 808,
					},
					AccessLogs: &api.AccessLogsConfig{
						Enabled:             true,
						DisableListenerLogs: true,
						Type:                api.FileLogSinkType,
						Path:                "/var/logs/envoy.logs",
						TextFormat:          "MY START TIME %START_TIME%",
					},
					Config: map[string]interface{}{
						"foo": "bar",
					},
				},
			},
		},
	}

	for _, tt := range cases {
		// Capture the loop variable locally otherwise parallel will cause us to run
		// N copies of the last test case but with different names!!
		tc := tt
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			actual, err := serviceToAgentService(tc.Input)
			require.NoError(t, err)
			require.Equal(t, tc.Output, actual)
		})
	}
}
