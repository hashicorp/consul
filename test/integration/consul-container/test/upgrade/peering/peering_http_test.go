package peering

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
	"github.com/hashicorp/consul/test/integration/consul-container/test/upgrade"
)

// TestPeering_UpgradeToTarget_fromLatest checks peering status after dialing cluster
// and accepting cluster upgrade
func TestPeering_UpgradeToTarget_fromLatest(t *testing.T) {
	t.Parallel()

	type testcase struct {
		oldversion    string
		targetVersion string
		name          string
		// create creates addtional resources in peered clusters depending on cases, e.g., static-client,
		// static server, and config-entries. It returns the proxy services, an assertation function to
		// be called to verify the resources.
		create func(*cluster.Cluster, *cluster.Cluster) (libservice.Service, libservice.Service, func(), error)
		// extraAssertion adds additional assertion function to the common resources across cases.
		// common resources includes static-client in dialing cluster, and static-server in accepting cluster.
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
			create: func(accepting *cluster.Cluster, dialing *cluster.Cluster) (libservice.Service, libservice.Service, func(), error) {
				return nil, nil, func() {}, nil
			},
			extraAssertion: func(clientUpstreamPort int) {},
		},
		{
			oldversion:    "1.14",
			targetVersion: utils.TargetVersion,
			name:          "http_router",
			// Create a second static-service at the client agent of accepting cluster and
			// a service-router that routes /static-server-2 to static-server-2
			create: func(accepting *cluster.Cluster, dialing *cluster.Cluster) (libservice.Service, libservice.Service, func(), error) {
				c := accepting
				serviceOpts := &libservice.ServiceOpts{
					Name:     libservice.StaticServer2ServiceName,
					ID:       "static-server-2",
					Meta:     map[string]string{"version": "v2"},
					HTTPPort: 8081,
					GRPCPort: 8078,
				}
				_, serverConnectProxy, err := libservice.CreateAndRegisterStaticServerAndSidecar(c.Clients()[0], serviceOpts)
				if err != nil {
					return nil, nil, nil, err
				}
				libassert.CatalogServiceExists(t, c.Clients()[0].GetClient(), libservice.StaticServer2ServiceName, nil)

				err = c.ConfigEntryWrite(&api.ProxyConfigEntry{
					Kind: api.ProxyDefaults,
					Name: "global",
					Config: map[string]interface{}{
						"protocol": "http",
					},
				})
				if err != nil {
					return nil, nil, nil, err
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
				return serverConnectProxy, nil, func() {}, err
			},
			extraAssertion: func(clientUpstreamPort int) {
				libassert.AssertFortioName(t, fmt.Sprintf("http://localhost:%d/static-server-2", clientUpstreamPort), "static-server-2", "")
			},
		},
		{
			oldversion:    "1.14",
			targetVersion: utils.TargetVersion,
			name:          "http splitter and resolver",
			// In addtional to the basic topology, this case provisions the following
			// services in the dialing cluster:
			//
			// - a new static-client at server_0 that has two upstreams: split-static-server (5000)
			//   and peer-static-server (5001)
			// - a local static-server service at client_0
			// - service-splitter named split-static-server w/ 2 services: "local-static-server" and
			//   "peer-static-server".
			// - service-resolved named local-static-server
			// - service-resolved named peer-static-server
			create: func(accepting *cluster.Cluster, dialing *cluster.Cluster) (libservice.Service, libservice.Service, func(), error) {
				err := dialing.ConfigEntryWrite(&api.ProxyConfigEntry{
					Kind: api.ProxyDefaults,
					Name: "global",
					Config: map[string]interface{}{
						"protocol": "http",
					},
				})
				if err != nil {
					return nil, nil, nil, err
				}

				clientConnectProxy, err := upgrade.CreateAndRegisterStaticClientSidecarWith2Upstreams(dialing,
					[]string{"split-static-server", "peer-static-server"},
					true,
				)
				if err != nil {
					return nil, nil, nil, fmt.Errorf("error creating client connect proxy in cluster %s", dialing.NetworkName)
				}

				// make a resolver for service peer-static-server
				resolverConfigEntry := &api.ServiceResolverConfigEntry{
					Kind: api.ServiceResolver,
					Name: "peer-static-server",
					Redirect: &api.ServiceResolverRedirect{
						Service: libservice.StaticServerServiceName,
						Peer:    libtopology.DialingPeerName,
					},
				}
				err = dialing.ConfigEntryWrite(resolverConfigEntry)
				if err != nil {
					return nil, nil, nil, fmt.Errorf("error writing resolver config entry for %s", resolverConfigEntry.Name)
				}

				// make a splitter for service split-static-server
				splitter := &api.ServiceSplitterConfigEntry{
					Kind: api.ServiceSplitter,
					Name: "split-static-server",
					Splits: []api.ServiceSplit{
						{
							Weight:  50,
							Service: "local-static-server",
							ResponseHeaders: &api.HTTPHeaderModifiers{
								Set: map[string]string{
									"x-test-split": "local",
								},
							},
						},
						{
							Weight:  50,
							Service: "peer-static-server",
							ResponseHeaders: &api.HTTPHeaderModifiers{
								Set: map[string]string{
									"x-test-split": "peer",
								},
							},
						},
					},
				}
				err = dialing.ConfigEntryWrite(splitter)
				if err != nil {
					return nil, nil, nil, fmt.Errorf("error writing splitter config entry for %s", splitter.Name)
				}

				// make a resolver for service local-static-server
				resolverConfigEntry = &api.ServiceResolverConfigEntry{
					Kind: api.ServiceResolver,
					Name: "local-static-server",
					Redirect: &api.ServiceResolverRedirect{
						Service: libservice.StaticServerServiceName,
					},
				}
				err = dialing.ConfigEntryWrite(resolverConfigEntry)
				if err != nil {
					return nil, nil, nil, fmt.Errorf("error writing resolver config entry for %s", resolverConfigEntry.Name)
				}

				// Make a static-server in dialing cluster
				serviceOpts := &libservice.ServiceOpts{
					Name:     libservice.StaticServerServiceName,
					ID:       "static-server",
					HTTPPort: 8081,
					GRPCPort: 8078,
				}
				_, serverConnectProxy, err := libservice.CreateAndRegisterStaticServerAndSidecar(dialing.Clients()[0], serviceOpts)
				libassert.CatalogServiceExists(t, dialing.Clients()[0].GetClient(), libservice.StaticServerServiceName, nil)
				if err != nil {
					return nil, nil, nil, err
				}

				_, appPorts := clientConnectProxy.GetAddrs()
				assertionFn := func() {
					libassert.HTTPServiceEchoesResHeader(t, "localhost", appPorts[0], "", map[string]string{
						"X-Test-Split": "local",
					})
					libassert.HTTPServiceEchoesResHeader(t, "localhost", appPorts[0], "", map[string]string{
						"X-Test-Split": "peer",
					})
					libassert.HTTPServiceEchoes(t, "localhost", appPorts[0], "")
				}
				return serverConnectProxy, clientConnectProxy, assertionFn, nil
			},
			extraAssertion: func(clientUpstreamPort int) {},
		},
		{
			oldversion:    "1.14",
			targetVersion: utils.TargetVersion,
			name:          "http resolver and failover",
			// Verify resolver and failover can direct traffic to server in peered cluster
			// In addtional to the basic topology, this case provisions the following
			// services in the dialing cluster:
			//
			// - a new static-client at server_0 that has two upstreams: static-server (5000)
			//   and peer-static-server (5001)
			// - a local static-server service at client_0
			// - service-resolved named static-server with failover to static-server in accepting cluster
			// - service-resolved named peer-static-server to static-server in accepting cluster
			create: func(accepting *cluster.Cluster, dialing *cluster.Cluster) (libservice.Service, libservice.Service, func(), error) {
				err := dialing.ConfigEntryWrite(&api.ProxyConfigEntry{
					Kind: api.ProxyDefaults,
					Name: "global",
					Config: map[string]interface{}{
						"protocol": "http",
					},
				})
				if err != nil {
					return nil, nil, nil, err
				}

				clientConnectProxy, err := upgrade.CreateAndRegisterStaticClientSidecarWith2Upstreams(dialing,
					[]string{"static-server", "peer-static-server"}, true,
				)
				if err != nil {
					return nil, nil, nil, fmt.Errorf("error creating client connect proxy in cluster %s", dialing.NetworkName)
				}

				// make a resolver for service peer-static-server
				resolverConfigEntry := &api.ServiceResolverConfigEntry{
					Kind: api.ServiceResolver,
					Name: "peer-static-server",
					Redirect: &api.ServiceResolverRedirect{
						Service: libservice.StaticServerServiceName,
						Peer:    libtopology.DialingPeerName,
					},
				}
				err = dialing.ConfigEntryWrite(resolverConfigEntry)
				if err != nil {
					return nil, nil, nil, fmt.Errorf("error writing resolver config entry for %s", resolverConfigEntry.Name)
				}

				// make a resolver for service static-server
				resolverConfigEntry = &api.ServiceResolverConfigEntry{
					Kind: api.ServiceResolver,
					Name: "static-server",
					Failover: map[string]api.ServiceResolverFailover{
						"*": {
							Targets: []api.ServiceResolverFailoverTarget{
								{
									Peer: libtopology.DialingPeerName,
								},
							},
						},
					},
				}
				err = dialing.ConfigEntryWrite(resolverConfigEntry)
				if err != nil {
					return nil, nil, nil, fmt.Errorf("error writing resolver config entry for %s", resolverConfigEntry.Name)
				}

				// Make a static-server in dialing cluster
				serviceOpts := &libservice.ServiceOpts{
					Name:     libservice.StaticServerServiceName,
					ID:       "static-server-dialing",
					HTTPPort: 8081,
					GRPCPort: 8078,
				}
				_, serverConnectProxy, err := libservice.CreateAndRegisterStaticServerAndSidecar(dialing.Clients()[0], serviceOpts)
				libassert.CatalogServiceExists(t, dialing.Clients()[0].GetClient(), libservice.StaticServerServiceName, nil)
				if err != nil {
					return nil, nil, nil, err
				}

				_, appPorts := clientConnectProxy.GetAddrs()
				assertionFn := func() {
					// assert traffic can fail-over to static-server in peered cluster and restor to local static-server
					libassert.AssertFortioName(t, fmt.Sprintf("http://localhost:%d", appPorts[0]), "static-server-dialing", "")
					require.NoError(t, serverConnectProxy.Stop())
					libassert.AssertFortioName(t, fmt.Sprintf("http://localhost:%d", appPorts[0]), "static-server", "")
					require.NoError(t, serverConnectProxy.Start())
					libassert.AssertFortioName(t, fmt.Sprintf("http://localhost:%d", appPorts[0]), "static-server-dialing", "")

					// assert peer-static-server resolves to static-server in peered cluster
					libassert.AssertFortioName(t, fmt.Sprintf("http://localhost:%d", appPorts[1]), "static-server", "")
				}
				return serverConnectProxy, clientConnectProxy, assertionFn, nil
			},
			extraAssertion: func(clientUpstreamPort int) {},
		},
	}

	run := func(t *testing.T, tc testcase) {
		accepting, dialing := libtopology.BasicPeeringTwoClustersSetup(t, utils.GetLatestImageName(), tc.oldversion,
			libtopology.PeeringClusterSize{
				AcceptingNumServers: 1,
				AcceptingNumClients: 1,
				DialingNumServers:   1,
				DialingNumClients:   1,
			},
			false)
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
		_, secondClientProxy, assertionAdditionalResources, err := tc.create(acceptingCluster, dialingCluster)
		require.NoError(t, err)
		assertionAdditionalResources()
		tc.extraAssertion(appPort)

		// Upgrade the accepting cluster and assert peering is still ACTIVE
		require.NoError(t, acceptingCluster.StandardUpgrade(t, context.Background(), utils.GetTargetImageName(), tc.targetVersion))
		libassert.PeeringStatus(t, acceptingClient, libtopology.AcceptingPeerName, api.PeeringStateActive)
		libassert.PeeringStatus(t, dialingClient, libtopology.DialingPeerName, api.PeeringStateActive)

		require.NoError(t, dialingCluster.StandardUpgrade(t, context.Background(), utils.GetTargetImageName(), tc.targetVersion))
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

		// restart the secondClientProxy if exist
		if secondClientProxy != nil {
			require.NoError(t, secondClientProxy.Restart())
		}
		assertionAdditionalResources()

		clientSidecarService, err := libservice.CreateAndRegisterStaticClientSidecar(dialingCluster.Servers()[0], libtopology.DialingPeerName, true)
		require.NoError(t, err)
		_, port := clientSidecarService.GetAddr()
		_, adminPort := clientSidecarService.GetAdminAddr()
		libassert.AssertUpstreamEndpointStatus(t, adminPort, fmt.Sprintf("static-server.default.%s.external", libtopology.DialingPeerName), "HEALTHY", 1)
		libassert.HTTPServiceEchoes(t, "localhost", port, "")
		libassert.AssertFortioName(t, fmt.Sprintf("http://localhost:%d", port), "static-server", "")

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
