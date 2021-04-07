package redirecttraffic

import (
	"testing"

	"github.com/hashicorp/consul/agent/xds"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/iptables"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestRun_FlagValidation(t *testing.T) {
	t.Parallel()

	ui := cli.NewMockUi()
	c := New(ui)

	code := c.Run(nil)
	require.Equal(t, code, 1)
	require.Contains(t, ui.ErrorWriter.String(), "-proxy-uid is required")
}

func TestGenerateConfigFromFlags(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name             string
		command          cmd
		createTestServer bool
		expCfg           iptables.Config
		expError         string
	}{
		{
			"proxyID provided",
			cmd{
				proxyUID: "1234",
				proxyID:  "test-proxy-id",
			},
			true,
			iptables.Config{
				ProxyUserID:       "1234",
				ProxyInboundPort:  20000,
				ProxyOutboundPort: xds.TProxyOutboundPort,
			},
			"",
		},
		{
			"proxyID provided, but Consul is not reachable",
			cmd{
				proxyUID: "1234",
				proxyID:  "test-proxy-id",
			},
			false,
			iptables.Config{},
			"failed to fetch proxy service from Consul Agent: ",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.createTestServer {
				testServer, err := testutil.NewTestServerConfigT(t, nil)
				require.NoError(t, err)
				defer testServer.Stop()

				client, err := api.NewClient(&api.Config{Address: testServer.HTTPAddr})
				require.NoError(t, err)

				err = client.Agent().ServiceRegister(&api.AgentServiceRegistration{
					Kind:    api.ServiceKindConnectProxy,
					ID:      c.command.proxyID,
					Name:    "test-proxy",
					Port:    20000,
					Address: "1.1.1.1",
					Proxy: &api.AgentServiceConnectProxyConfig{
						DestinationServiceName: "foo",
					},
				})
				require.NoError(t, err)

				c.command.client = client
			} else {
				client, err := api.NewClient(&api.Config{Address: "not-reachable"})
				require.NoError(t, err)
				c.command.client = client
			}

			cfg, err := c.command.generateConfigFromFlags()

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
