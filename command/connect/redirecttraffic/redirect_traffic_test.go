package redirecttraffic

import (
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/iptables"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestRun_FlagValidation(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	cases := []struct {
		name           string
		command        func() cmd
		consulServices []api.AgentServiceRegistration
		expCfg         iptables.Config
		expError       string
	}{
		{
			"proxyID with service port provided",
			func() cmd {
				var c cmd
				c.init()
				c.proxyUID = "1234"
				c.proxyID = "test-proxy-id"
				return c
			},
			[]api.AgentServiceRegistration{
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
			iptables.Config{
				ProxyUserID:       "1234",
				ProxyInboundPort:  20000,
				ProxyOutboundPort: iptables.DefaultTProxyOutboundPort,
			},
			"",
		},
		{
			"proxyID with bind_port(int) provided",
			func() cmd {
				var c cmd
				c.init()
				c.proxyUID = "1234"
				c.proxyID = "test-proxy-id"
				return c
			},
			[]api.AgentServiceRegistration{
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
			iptables.Config{
				ProxyUserID:       "1234",
				ProxyInboundPort:  21000,
				ProxyOutboundPort: iptables.DefaultTProxyOutboundPort,
			},
			"",
		},
		{
			"proxyID with bind_port(string) provided",
			func() cmd {
				var c cmd
				c.init()
				c.proxyUID = "1234"
				c.proxyID = "test-proxy-id"
				return c
			},
			[]api.AgentServiceRegistration{
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
			iptables.Config{
				ProxyUserID:       "1234",
				ProxyInboundPort:  21000,
				ProxyOutboundPort: iptables.DefaultTProxyOutboundPort,
			},
			"",
		},
		{
			"proxyID with bind_port(invalid type) provided",
			func() cmd {
				var c cmd
				c.init()
				c.proxyUID = "1234"
				c.proxyID = "test-proxy-id"
				return c
			},
			[]api.AgentServiceRegistration{
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
			iptables.Config{},
			"failed parsing Proxy.Config: 1 error(s) decoding:\n\n* cannot parse 'bind_port' as int:",
		},
		{
			"proxyID with proxy outbound port",
			func() cmd {
				var c cmd
				c.init()
				c.proxyUID = "1234"
				c.proxyID = "test-proxy-id"
				return c
			},
			[]api.AgentServiceRegistration{
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
			iptables.Config{
				ProxyUserID:       "1234",
				ProxyInboundPort:  20000,
				ProxyOutboundPort: 21000,
			},
			"",
		},
		{
			"proxyID provided, but Consul is not reachable",
			func() cmd {
				var c cmd
				c.init()
				c.proxyUID = "1234"
				c.proxyID = "test-proxy-id"
				return c
			},
			nil,
			iptables.Config{},
			"failed to fetch proxy service from Consul Agent: ",
		},
		{
			"proxyID of a non-proxy service",
			func() cmd {
				var c cmd
				c.init()
				c.proxyUID = "1234"
				c.proxyID = "test-proxy-id"
				return c
			},
			[]api.AgentServiceRegistration{
				{
					ID:      "test-proxy-id",
					Name:    "test-proxy",
					Port:    20000,
					Address: "1.1.1.1",
				},
			},
			iptables.Config{},
			"service test-proxy-id is not a proxy service",
		},
		{
			"only proxy inbound port is provided",
			func() cmd {
				var c cmd
				c.init()
				c.proxyUID = "1234"
				c.proxyInboundPort = 15000
				return c
			},
			nil,
			iptables.Config{
				ProxyUserID:       "1234",
				ProxyInboundPort:  15000,
				ProxyOutboundPort: iptables.DefaultTProxyOutboundPort,
			},
			"",
		},
		{
			"proxy inbound and outbound ports are provided",
			func() cmd {
				var c cmd
				c.init()
				c.proxyUID = "1234"
				c.proxyInboundPort = 15000
				c.proxyOutboundPort = 16000
				return c
			},
			nil,
			iptables.Config{
				ProxyUserID:       "1234",
				ProxyInboundPort:  15000,
				ProxyOutboundPort: 16000,
			},
			"",
		},
		{
			"exclude inbound ports are provided",
			func() cmd {
				var c cmd
				c.init()
				c.proxyUID = "1234"
				c.proxyInboundPort = 15000
				c.excludeInboundPorts = []string{"8080", "21000"}
				return c
			},
			nil,
			iptables.Config{
				ProxyUserID:         "1234",
				ProxyInboundPort:    15000,
				ProxyOutboundPort:   15001,
				ExcludeInboundPorts: []string{"8080", "21000"},
			},
			"",
		},
		{
			"exclude outbound ports are provided",
			func() cmd {
				var c cmd
				c.init()
				c.proxyUID = "1234"
				c.proxyInboundPort = 15000
				c.excludeOutboundPorts = []string{"8080", "21000"}
				return c
			},
			nil,
			iptables.Config{
				ProxyUserID:          "1234",
				ProxyInboundPort:     15000,
				ProxyOutboundPort:    15001,
				ExcludeOutboundPorts: []string{"8080", "21000"},
			},
			"",
		},
		{
			"exclude outbound CIDRs are provided",
			func() cmd {
				var c cmd
				c.init()
				c.proxyUID = "1234"
				c.proxyInboundPort = 15000
				c.excludeOutboundCIDRs = []string{"1.1.1.1", "2.2.2.2/24"}
				return c
			},
			nil,
			iptables.Config{
				ProxyUserID:          "1234",
				ProxyInboundPort:     15000,
				ProxyOutboundPort:    15001,
				ExcludeOutboundCIDRs: []string{"1.1.1.1", "2.2.2.2/24"},
			},
			"",
		},
		{
			"exclude UIDs are provided",
			func() cmd {
				var c cmd
				c.init()
				c.proxyUID = "1234"
				c.proxyInboundPort = 15000
				c.excludeUIDs = []string{"2345", "3456"}
				return c
			},
			nil,
			iptables.Config{
				ProxyUserID:       "1234",
				ProxyInboundPort:  15000,
				ProxyOutboundPort: 15001,
				ExcludeUIDs:       []string{"2345", "3456"},
			},
			"",
		},
		{
			"proxy config has envoy_prometheus_bind_addr set",
			func() cmd {
				var c cmd
				c.init()
				c.proxyUID = "1234"
				c.proxyID = "test-proxy-id"
				return c
			},
			[]api.AgentServiceRegistration{
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
			iptables.Config{
				ProxyUserID:         "1234",
				ProxyInboundPort:    20000,
				ProxyOutboundPort:   iptables.DefaultTProxyOutboundPort,
				ExcludeInboundPorts: []string{"9000"},
			},
			"",
		},
		{
			"proxy config has an invalid envoy_prometheus_bind_addr set",
			func() cmd {
				var c cmd
				c.init()
				c.proxyUID = "1234"
				c.proxyID = "test-proxy-id"
				return c
			},
			[]api.AgentServiceRegistration{
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
			iptables.Config{},
			"failed parsing host and port from envoy_prometheus_bind_addr: address 9000: missing port in address",
		},
		{
			"proxy config has envoy_stats_bind_addr set",
			func() cmd {
				var c cmd
				c.init()
				c.proxyUID = "1234"
				c.proxyID = "test-proxy-id"
				return c
			},
			[]api.AgentServiceRegistration{
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
			iptables.Config{
				ProxyUserID:         "1234",
				ProxyInboundPort:    20000,
				ProxyOutboundPort:   iptables.DefaultTProxyOutboundPort,
				ExcludeInboundPorts: []string{"8000"},
			},
			"",
		},
		{
			"proxy config has an invalid envoy_stats_bind_addr set",
			func() cmd {
				var c cmd
				c.init()
				c.proxyUID = "1234"
				c.proxyID = "test-proxy-id"
				return c
			},
			[]api.AgentServiceRegistration{
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
			iptables.Config{},
			"failed parsing host and port from envoy_stats_bind_addr: address 8000: missing port in address",
		},
		{
			"proxy config has expose paths with listener port set",
			func() cmd {
				var c cmd
				c.init()
				c.proxyUID = "1234"
				c.proxyID = "test-proxy-id"
				return c
			},
			[]api.AgentServiceRegistration{
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
			iptables.Config{
				ProxyUserID:         "1234",
				ProxyInboundPort:    20000,
				ProxyOutboundPort:   iptables.DefaultTProxyOutboundPort,
				ExcludeInboundPorts: []string{"23000"},
			},
			"",
		},
		{
			"proxy config has expose paths with checks set to true",
			func() cmd {
				var c cmd
				c.init()
				c.proxyUID = "1234"
				c.proxyID = "test-proxy-id"
				return c
			},
			[]api.AgentServiceRegistration{
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
			iptables.Config{
				ProxyUserID:         "1234",
				ProxyInboundPort:    20000,
				ProxyOutboundPort:   iptables.DefaultTProxyOutboundPort,
				ExcludeInboundPorts: []string{"21500", "21501"},
			},
			"",
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
				require.Equal(t, c.expCfg, cfg)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), c.expError)
			}
		})
	}
}
