package upgrade

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

// TestPeering_UpgradeToTarget_fromLatest checks peering status after dialing cluster
// and accepting cluster upgrade
func TestPeering_UpgradeToTarget_fromLatest(t *testing.T) {
	t.Parallel()

	type testcase struct {
		oldversion     string
		targetVersion  string
		name           string
		create         func(*cluster.Cluster) (libservice.Service, error)
		extraAssertion func(int)
	}
	tcs := []testcase{
		// {
		//  TODO: API changed from 1.13 to 1.14 in , PeerName to Peer
		//  exportConfigEntry
		// 	oldversion:    "1.13",
		// 	targetVersion: *utils.TargetVersion,
		// },
		{
			oldversion:    "1.14",
			targetVersion: utils.TargetVersion,
			name:          "basic",
			create: func(c *cluster.Cluster) (libservice.Service, error) {
				return nil, nil
			},
			extraAssertion: func(clientUpstreamPort int) {},
		},
		{
			oldversion:    "1.14",
			targetVersion: utils.TargetVersion,
			name:          "http_router",
			// Create a second static-service at the client agent of accepting cluster and
			// a service-router that routes /static-server-2 to static-server-2
			create: func(c *cluster.Cluster) (libservice.Service, error) {
				serviceOpts := &libservice.ServiceOpts{
					Name:     libservice.StaticServer2ServiceName,
					ID:       "static-server-2",
					Meta:     map[string]string{"version": "v2"},
					HTTPPort: 8081,
					GRPCPort: 8078,
				}
				_, serverConnectProxy, err := libservice.CreateAndRegisterStaticServerAndSidecar(c.Clients()[0], serviceOpts)
				libassert.CatalogServiceExists(t, c.Clients()[0].GetClient(), libservice.StaticServer2ServiceName)
				if err != nil {
					return nil, err
				}
				err = c.ConfigEntryWrite(&api.ProxyConfigEntry{
					Kind: api.ProxyDefaults,
					Name: "global",
					Config: map[string]interface{}{
						"protocol": "http",
					},
				})
				if err != nil {
					return nil, err
				}
				routerConfigEntry := &api.ServiceRouterConfigEntry{
					Kind: api.ServiceRouter,
					Name: libservice.StaticServerServiceName,
					Routes: []api.ServiceRoute{
						{
							Match: &api.ServiceRouteMatch{
								HTTP: &api.ServiceRouteHTTPMatch{
									PathPrefix: "/" + libservice.StaticServer2ServiceName + "/",
								},
							},
							Destination: &api.ServiceRouteDestination{
								Service:       libservice.StaticServer2ServiceName,
								PrefixRewrite: "/",
							},
						},
					},
				}
				err = c.ConfigEntryWrite(routerConfigEntry)
				return serverConnectProxy, err
			},
			extraAssertion: func(clientUpstreamPort int) {
				libassert.AssertFortioName(t, fmt.Sprintf("http://localhost:%d/static-server-2", clientUpstreamPort), "static-server-2")
			},
		},
	}

	run := func(t *testing.T, tc testcase) {
		accepting, dialing := libtopology.BasicPeeringTwoClustersSetup(t, tc.oldversion, false)
		var (
			acceptingCluster = accepting.Cluster
			dialingCluster   = dialing.Cluster
		)

		dialingClient, err := dialingCluster.GetClient(nil, false)
		require.NoError(t, err)

		acceptingClient, err := acceptingCluster.GetClient(nil, false)
		require.NoError(t, err)

		_, gatewayAdminPort := dialing.Gateway.GetAdminAddr()
		_, staticClientPort := dialing.Container.GetAddr()

		_, appPort := dialing.Container.GetAddr()
		_, err = tc.create(acceptingCluster)
		require.NoError(t, err)
		tc.extraAssertion(appPort)

		// Upgrade the accepting cluster and assert peering is still ACTIVE
		require.NoError(t, acceptingCluster.StandardUpgrade(t, context.Background(), tc.targetVersion))
		libassert.PeeringStatus(t, acceptingClient, libtopology.AcceptingPeerName, api.PeeringStateActive)
		libassert.PeeringStatus(t, dialingClient, libtopology.DialingPeerName, api.PeeringStateActive)

		require.NoError(t, dialingCluster.StandardUpgrade(t, context.Background(), tc.targetVersion))
		libassert.PeeringStatus(t, acceptingClient, libtopology.AcceptingPeerName, api.PeeringStateActive)
		libassert.PeeringStatus(t, dialingClient, libtopology.DialingPeerName, api.PeeringStateActive)

		// POST upgrade validation
		//  - Register a new static-client service in dialing cluster and
		//  - set upstream to static-server service in peered cluster

		// Restart the gateway & proxy sidecar, and verify existing connection
		require.NoError(t, dialing.Gateway.Restart())
		// Restarted gateway should not have any measurement on data plane traffic
		libassert.AssertEnvoyMetricAtMost(t, gatewayAdminPort,
			"cluster.static-server.default.default.accepting-to-dialer.external",
			"upstream_cx_total", 0)
		libassert.HTTPServiceEchoes(t, "localhost", staticClientPort, "")

		require.NoError(t, dialing.Container.Restart())
		libassert.HTTPServiceEchoes(t, "localhost", staticClientPort, "")
		require.NoError(t, accepting.Container.Restart())
		libassert.HTTPServiceEchoes(t, "localhost", staticClientPort, "")

		clientSidecarService, err := libservice.CreateAndRegisterStaticClientSidecar(dialingCluster.Servers()[0], libtopology.DialingPeerName, true)
		require.NoError(t, err)
		_, port := clientSidecarService.GetAddr()
		_, adminPort := clientSidecarService.GetAdminAddr()
		libassert.AssertUpstreamEndpointStatus(t, adminPort, fmt.Sprintf("static-server.default.%s.external", libtopology.DialingPeerName), "HEALTHY", 1)
		libassert.HTTPServiceEchoes(t, "localhost", port, "")
		libassert.AssertFortioName(t, fmt.Sprintf("http://localhost:%d", port), "static-server")

		// TODO: restart static-server-2's sidecar
		tc.extraAssertion(appPort)
	}

	for _, tc := range tcs {
		t.Run(fmt.Sprintf("%s upgrade from %s to %s", tc.name, tc.oldversion, tc.targetVersion),
			func(t *testing.T) {
				run(t, tc)
			})
		// time.Sleep(3 * time.Second)
	}
}
