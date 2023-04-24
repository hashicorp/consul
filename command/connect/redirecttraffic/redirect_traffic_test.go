package redirecttraffic

import (
	"sort"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/iptables"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestRun_FlagValidation(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		expError string
	}{
		{
			"-proxy-uid is missing",
			nil,
			"-proxy-uid is required",
		},
		{
			"-proxy-id and -proxy-inbound-port are missing",
			[]string{"-proxy-uid=1234"},
			"either -proxy-id or -proxy-inbound-port are required",
		},
		{
			"-proxy-id and -proxy-inbound-port are provided",
			[]string{"-proxy-uid=1234", "-proxy-id=test", "-proxy-inbound-port=15000"},
			"-proxy-inbound-port or -proxy-outbound-port cannot be provided together with -proxy-id.",
		},
		{
			"-proxy-id and -proxy-outbound-port are provided",
			[]string{"-proxy-uid=1234", "-proxy-id=test", "-proxy-outbound-port=15000"},
			"-proxy-inbound-port or -proxy-outbound-port cannot be provided together with -proxy-id.",
		},
		{
			"-proxy-id, -proxy-inbound-port and non-default -proxy-outbound-port are provided",
			[]string{"-proxy-uid=1234", "-proxy-id=test", "-proxy-inbound-port=15000", "-proxy-outbound-port=15001"},
			"-proxy-inbound-port or -proxy-outbound-port cannot be provided together with -proxy-id.",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ui := cli.NewMockUi()
			cmd := New(ui)

			code := cmd.Run(c.args)
			require.Equal(t, code, 1)
			require.Contains(t, ui.ErrorWriter.String(), c.expError)
		})
	}

}

func TestGenerateConfigFromFlags(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	cases := []struct {
		name           string
		command        func() cmd
		consulServices []api.AgentServiceRegistration
		expCfg         iptables.Config
		expError       string
	}{
		{
			name: "proxyID with service port provided",
			command: func() cmd {
				var c cmd
				c.init()
				c.proxyUID = "1234"
				c.proxyID = "test-proxy-id"
				return c
			},
			consulServices: []api.AgentServiceRegistration{
				{
					Kind:    api.ServiceKindConnectProxy,
					ID:      "test-proxy-id",
					Name:    "test-proxy",
					Port:    20000,
					Address: "1.1.1.1",
					Proxy: &api.AgentServiceConnectProxyConfig{
						DestinationServiceName: "foo",
					},
				},
			},
			expCfg: iptables.Config{
				ProxyUserID:       "1234",
				ProxyInboundPort:  20000,
				ProxyOutboundPort: iptables.DefaultTProxyOutboundPort,
			},
		},
		{
			name: "proxyID with bind_port(int) provided",
			command: func() cmd {
				var c cmd
				c.init()
				c.proxyUID = "1234"
				c.proxyID = "test-proxy-id"
				return c
			},
			consulServices: []api.AgentServiceRegistration{
				{
					Kind:    api.ServiceKindConnectProxy,
					ID:      "test-proxy-id",
					Name:    "test-proxy",
					Port:    20000,
					Address: "1.1.1.1",
					Proxy: &api.AgentServiceConnectProxyConfig{
						DestinationServiceName: "foo",
						Config: map[string]interface{}{
							"bind_port": 21000,
						},
					},
				},
			},
			expCfg: iptables.Config{
				ProxyUserID:       "1234",
				ProxyInboundPort:  21000,
				ProxyOutboundPort: iptables.DefaultTProxyOutboundPort,
			},
		},
		{
			name: "proxyID with Consul DNS IP provided",
			command: func() cmd {
				var c cmd
				c.init()
				c.proxyUID = "1234"
				c.proxyID = "test-proxy-id"
				c.consulDNSIP = "10.0.34.16"
				return c
			},
			consulServices: []api.AgentServiceRegistration{
				{
					Kind:    api.ServiceKindConnectProxy,
					ID:      "test-proxy-id",
					Name:    "test-proxy",
					Port:    20000,
					Address: "1.1.1.1",
					Proxy: &api.AgentServiceConnectProxyConfig{
						DestinationServiceName: "foo",
					},
				},
			},
			expCfg: iptables.Config{
				ConsulDNSIP:       "10.0.34.16",
				ProxyUserID:       "1234",
				ProxyInboundPort:  20000,
				ProxyOutboundPort: iptables.DefaultTProxyOutboundPort,
			},
		},
		{
			name: "proxyID with bind_port(string) provided",
			command: func() cmd {
				var c cmd
				c.init()
				c.proxyUID = "1234"
				c.proxyID = "test-proxy-id"
				return c
			},
			consulServices: []api.AgentServiceRegistration{
				{
					Kind:    api.ServiceKindConnectProxy,
					ID:      "test-proxy-id",
					Name:    "test-proxy",
					Port:    20000,
					Address: "1.1.1.1",
					Proxy: &api.AgentServiceConnectProxyConfig{
						DestinationServiceName: "foo",
						Config: map[string]interface{}{
							"bind_port": "21000",
						},
					},
				},
			},
			expCfg: iptables.Config{
				ProxyUserID:       "1234",
				ProxyInboundPort:  21000,
				ProxyOutboundPort: iptables.DefaultTProxyOutboundPort,
			},
		},
		{
			name: "proxyID with bind_port(invalid type) provided",
			command: func() cmd {
				var c cmd
				c.init()
				c.proxyUID = "1234"
				c.proxyID = "test-proxy-id"
				return c
			},
			consulServices: []api.AgentServiceRegistration{
				{
					Kind:    api.ServiceKindConnectProxy,
					ID:      "test-proxy-id",
					Name:    "test-proxy",
					Port:    20000,
					Address: "1.1.1.1",
					Proxy: &api.AgentServiceConnectProxyConfig{
						DestinationServiceName: "foo",
						Config: map[string]interface{}{
							"bind_port": "invalid",
						},
					},
				},
			},
			expError: "failed parsing Proxy.Config: 1 error(s) decoding:\n\n* cannot parse 'bind_port' as int:",
		},
		{
			name: "proxyID with proxy outbound port",
			command: func() cmd {
				var c cmd
				c.init()
				c.proxyUID = "1234"
				c.proxyID = "test-proxy-id"
				return c
			},
			consulServices: []api.AgentServiceRegistration{
				{
					Kind:    api.ServiceKindConnectProxy,
					ID:      "test-proxy-id",
					Name:    "test-proxy",
					Port:    20000,
					Address: "1.1.1.1",
					Proxy: &api.AgentServiceConnectProxyConfig{
						DestinationServiceName: "foo",
						TransparentProxy: &api.TransparentProxyConfig{
							OutboundListenerPort: 21000,
						},
					},
				},
			},
			expCfg: iptables.Config{
				ProxyUserID:       "1234",
				ProxyInboundPort:  20000,
				ProxyOutboundPort: 21000,
			},
		},
		{
			name: "proxyID provided, but Consul is not reachable",
			command: func() cmd {
				var c cmd
				c.init()
				c.proxyUID = "1234"
				c.proxyID = "test-proxy-id"
				return c
			},
			expError: "failed to fetch proxy service from Consul Agent: ",
		},
		{
			name: "proxyID of a non-proxy service",
			command: func() cmd {
				var c cmd
				c.init()
				c.proxyUID = "1234"
				c.proxyID = "test-proxy-id"
				return c
			},
			consulServices: []api.AgentServiceRegistration{
				{
					ID:      "test-proxy-id",
					Name:    "test-proxy",
					Port:    20000,
					Address: "1.1.1.1",
				},
			},
			expError: "service test-proxy-id is not a proxy service",
		},
		{
			name: "only proxy inbound port is provided",
			command: func() cmd {
				var c cmd
				c.init()
				c.proxyUID = "1234"
				c.proxyInboundPort = 15000
				return c
			},
			expCfg: iptables.Config{
				ProxyUserID:       "1234",
				ProxyInboundPort:  15000,
				ProxyOutboundPort: iptables.DefaultTProxyOutboundPort,
			},
		},
		{
			name: "proxy inbound and outbound ports are provided",
			command: func() cmd {
				var c cmd
				c.init()
				c.proxyUID = "1234"
				c.proxyInboundPort = 15000
				c.proxyOutboundPort = 16000
				return c
			},
			expCfg: iptables.Config{
				ProxyUserID:       "1234",
				ProxyInboundPort:  15000,
				ProxyOutboundPort: 16000,
			},
		},
		{
			name: "exclude inbound ports are provided",
			command: func() cmd {
				var c cmd
				c.init()
				c.proxyUID = "1234"
				c.proxyInboundPort = 15000
				c.excludeInboundPorts = []string{"8080", "21000"}
				return c
			},
			expCfg: iptables.Config{
				ProxyUserID:         "1234",
				ProxyInboundPort:    15000,
				ProxyOutboundPort:   15001,
				ExcludeInboundPorts: []string{"8080", "21000"},
			},
		},
		{
			name: "exclude outbound ports are provided",
			command: func() cmd {
				var c cmd
				c.init()
				c.proxyUID = "1234"
				c.proxyInboundPort = 15000
				c.excludeOutboundPorts = []string{"8080", "21000"}
				return c
			},
			expCfg: iptables.Config{
				ProxyUserID:          "1234",
				ProxyInboundPort:     15000,
				ProxyOutboundPort:    15001,
				ExcludeOutboundPorts: []string{"8080", "21000"},
			},
		},
		{
			name: "exclude outbound CIDRs are provided",
			command: func() cmd {
				var c cmd
				c.init()
				c.proxyUID = "1234"
				c.proxyInboundPort = 15000
				c.excludeOutboundCIDRs = []string{"1.1.1.1", "2.2.2.2/24"}
				return c
			},
			expCfg: iptables.Config{
				ProxyUserID:          "1234",
				ProxyInboundPort:     15000,
				ProxyOutboundPort:    15001,
				ExcludeOutboundCIDRs: []string{"1.1.1.1", "2.2.2.2/24"},
			},
		},
		{
			name: "exclude UIDs are provided",
			command: func() cmd {
				var c cmd
				c.init()
				c.proxyUID = "1234"
				c.proxyInboundPort = 15000
				c.excludeUIDs = []string{"2345", "3456"}
				return c
			},
			expCfg: iptables.Config{
				ProxyUserID:       "1234",
				ProxyInboundPort:  15000,
				ProxyOutboundPort: 15001,
				ExcludeUIDs:       []string{"2345", "3456"},
			},
		},
		{
			name: "proxy config has envoy_prometheus_bind_addr set",
			command: func() cmd {
				var c cmd
				c.init()
				c.proxyUID = "1234"
				c.proxyID = "test-proxy-id"
				return c
			},
			consulServices: []api.AgentServiceRegistration{
				{
					Kind:    api.ServiceKindConnectProxy,
					ID:      "test-proxy-id",
					Name:    "test-proxy",
					Port:    20000,
					Address: "1.1.1.1",
					Proxy: &api.AgentServiceConnectProxyConfig{
						DestinationServiceName: "foo",
						Config: map[string]interface{}{
							"envoy_prometheus_bind_addr": "0.0.0.0:9000",
						},
					},
				},
			},
			expCfg: iptables.Config{
				ProxyUserID:         "1234",
				ProxyInboundPort:    20000,
				ProxyOutboundPort:   iptables.DefaultTProxyOutboundPort,
				ExcludeInboundPorts: []string{"9000"},
			},
		},
		{
			name: "proxy config has an invalid envoy_prometheus_bind_addr set",
			command: func() cmd {
				var c cmd
				c.init()
				c.proxyUID = "1234"
				c.proxyID = "test-proxy-id"
				return c
			},
			consulServices: []api.AgentServiceRegistration{
				{
					Kind:    api.ServiceKindConnectProxy,
					ID:      "test-proxy-id",
					Name:    "test-proxy",
					Port:    20000,
					Address: "1.1.1.1",
					Proxy: &api.AgentServiceConnectProxyConfig{
						DestinationServiceName: "foo",
						Config: map[string]interface{}{
							"envoy_prometheus_bind_addr": "9000",
						},
					},
				},
			},
			expError: "failed parsing host and port from envoy_prometheus_bind_addr: address 9000: missing port in address",
		},
		{
			name: "proxy config has envoy_stats_bind_addr set",
			command: func() cmd {
				var c cmd
				c.init()
				c.proxyUID = "1234"
				c.proxyID = "test-proxy-id"
				return c
			},
			consulServices: []api.AgentServiceRegistration{
				{
					Kind:    api.ServiceKindConnectProxy,
					ID:      "test-proxy-id",
					Name:    "test-proxy",
					Port:    20000,
					Address: "1.1.1.1",
					Proxy: &api.AgentServiceConnectProxyConfig{
						DestinationServiceName: "foo",
						Config: map[string]interface{}{
							"envoy_stats_bind_addr": "0.0.0.0:8000",
						},
					},
				},
			},
			expCfg: iptables.Config{
				ProxyUserID:         "1234",
				ProxyInboundPort:    20000,
				ProxyOutboundPort:   iptables.DefaultTProxyOutboundPort,
				ExcludeInboundPorts: []string{"8000"},
			},
		},
		{
			name: "proxy config has an invalid envoy_stats_bind_addr set",
			command: func() cmd {
				var c cmd
				c.init()
				c.proxyUID = "1234"
				c.proxyID = "test-proxy-id"
				return c
			},
			consulServices: []api.AgentServiceRegistration{
				{
					Kind:    api.ServiceKindConnectProxy,
					ID:      "test-proxy-id",
					Name:    "test-proxy",
					Port:    20000,
					Address: "1.1.1.1",
					Proxy: &api.AgentServiceConnectProxyConfig{
						DestinationServiceName: "foo",
						Config: map[string]interface{}{
							"envoy_stats_bind_addr": "8000",
						},
					},
				},
			},
			expError: "failed parsing host and port from envoy_stats_bind_addr: address 8000: missing port in address",
		},
		{
			name: "proxy config has expose paths with listener port set",
			command: func() cmd {
				var c cmd
				c.init()
				c.proxyUID = "1234"
				c.proxyID = "test-proxy-id"
				return c
			},
			consulServices: []api.AgentServiceRegistration{
				{
					Kind:    api.ServiceKindConnectProxy,
					ID:      "test-proxy-id",
					Name:    "test-proxy",
					Port:    20000,
					Address: "1.1.1.1",
					Proxy: &api.AgentServiceConnectProxyConfig{
						DestinationServiceName: "foo",
						Expose: api.ExposeConfig{
							Paths: []api.ExposePath{
								{
									ListenerPort:  23000,
									LocalPathPort: 8080,
									Path:          "/health",
								},
							},
						},
					},
				},
			},
			expCfg: iptables.Config{
				ProxyUserID:         "1234",
				ProxyInboundPort:    20000,
				ProxyOutboundPort:   iptables.DefaultTProxyOutboundPort,
				ExcludeInboundPorts: []string{"23000"},
			},
		},
		{
			name: "proxy config has expose paths with checks set to true",
			command: func() cmd {
				var c cmd
				c.init()
				c.proxyUID = "1234"
				c.proxyID = "test-proxy-id"
				return c
			},
			consulServices: []api.AgentServiceRegistration{
				{
					ID:      "foo-id",
					Name:    "foo",
					Port:    8080,
					Address: "1.1.1.1",
					Checks: []*api.AgentServiceCheck{
						{
							Name:     "http",
							HTTP:     "1.1.1.1:8080/health",
							Interval: "10s",
						},
						{
							Name:     "grpc",
							GRPC:     "1.1.1.1:8081",
							Interval: "10s",
						},
					},
				},
				{
					Kind:    api.ServiceKindConnectProxy,
					ID:      "test-proxy-id",
					Name:    "test-proxy",
					Port:    20000,
					Address: "1.1.1.1",
					Proxy: &api.AgentServiceConnectProxyConfig{
						DestinationServiceName: "foo",
						DestinationServiceID:   "foo-id",
						Expose: api.ExposeConfig{
							Checks: true,
						},
					},
				},
			},
			expCfg: iptables.Config{
				ProxyUserID:         "1234",
				ProxyInboundPort:    20000,
				ProxyOutboundPort:   iptables.DefaultTProxyOutboundPort,
				ExcludeInboundPorts: []string{"21500", "21501"},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cmd := c.command()
			if c.consulServices != nil {
				testServer, err := testutil.NewTestServerConfigT(t, nil)
				require.NoError(t, err)
				testServer.WaitForLeader(t)
				defer testServer.Stop()

				client, err := api.NewClient(&api.Config{Address: testServer.HTTPAddr})
				require.NoError(t, err)
				cmd.client = client

				for _, service := range c.consulServices {
					err = client.Agent().ServiceRegister(&service)
					require.NoError(t, err)
				}
			} else {
				client, err := api.NewClient(&api.Config{Address: "not-reachable"})
				require.NoError(t, err)
				cmd.client = client
			}

			cfg, err := cmd.generateConfigFromFlags()

			if c.expError == "" {
				require.NoError(t, err)

				sort.Strings(c.expCfg.ExcludeInboundPorts)
				sort.Strings(cfg.ExcludeInboundPorts)
				require.Equal(t, c.expCfg, cfg)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), c.expError)
			}
		})
	}
}
