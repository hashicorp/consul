// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package upgrade

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/hashicorp/consul/test/integration/consul-container/test/upgrade"
)

func TestPeering_Basic(t *testing.T) {
	t.Parallel()
	accepting, dialing := libtopology.BasicPeeringTwoClustersSetup(t, utils.GetLatestImageName(), utils.LatestVersion,
		libtopology.PeeringClusterSize{
			AcceptingNumServers: 1,
			AcceptingNumClients: 1,
			DialingNumServers:   1,
			DialingNumClients:   1,
		},
		false)
	peeringUpgrade(t, accepting, dialing, utils.TargetVersion)
	peeringPostUpgradeValidation(t, dialing)
}

func TestPeering_HTTPRouter(t *testing.T) {
	t.Parallel()
	accepting, dialing := libtopology.BasicPeeringTwoClustersSetup(t, utils.GetLatestImageName(), utils.LatestVersion,
		libtopology.PeeringClusterSize{
			AcceptingNumServers: 1,
			AcceptingNumClients: 1,
			DialingNumServers:   1,
			DialingNumClients:   1,
		},
		false)
	acceptingCluster := accepting.Cluster

	// Create a second static-server at the client agent of accepting cluster and
	// a service-router that routes /static-server-2 to static-server-2
	serviceOpts := &libservice.ServiceOpts{
		Name:     libservice.StaticServer2ServiceName,
		ID:       "static-server-2",
		Meta:     map[string]string{"version": "v2"},
		HTTPPort: 8081,
		GRPCPort: 8078,
	}
	_, _, err := libservice.CreateAndRegisterStaticServerAndSidecar(acceptingCluster.Clients()[0], serviceOpts)
	require.NoError(t, err, "creating static-server-2")
	libassert.CatalogServiceExists(t, acceptingCluster.Clients()[0].GetClient(), libservice.StaticServer2ServiceName, nil)

	require.NoError(t, acceptingCluster.ConfigEntryWrite(&api.ProxyConfigEntry{
		Kind: api.ProxyDefaults,
		Name: "global",
		Config: map[string]interface{}{
			"protocol": "http",
		},
	}))
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
	require.NoError(t, acceptingCluster.ConfigEntryWrite(routerConfigEntry))
	_, appPort := dialing.Container.GetAddr()
	libassert.WaitForFortioName(t, retry.ThirtySeconds(), fmt.Sprintf("http://localhost:%d/static-server-2", appPort), "static-server-2", "")

	peeringUpgrade(t, accepting, dialing, utils.TargetVersion)

	peeringPostUpgradeValidation(t, dialing)
	// TODO: restart static-server-2's sidecar
	libassert.WaitForFortioName(t, retry.ThirtySeconds(), fmt.Sprintf("http://localhost:%d/static-server-2", appPort), "static-server-2", "")
}

// Verify resolver and failover can direct traffic to server in peered cluster
// In addtional to the basic topology, this case provisions the following
// services in the dialing cluster:
//
//   - a new static-client at server_0 that has two upstreams: static-server (5000)
//     and peer-static-server (5001)
//   - a local static-server service at client_0
//   - service-resolved named static-server with failover to static-server in accepting cluster
//   - service-resolved named peer-static-server to static-server in accepting cluster
func TestPeering_HTTPResolverAndFailover(t *testing.T) {
	t.Parallel()

	accepting, dialing := libtopology.BasicPeeringTwoClustersSetup(t, utils.GetLatestImageName(), utils.LatestVersion,
		libtopology.PeeringClusterSize{
			AcceptingNumServers: 1,
			AcceptingNumClients: 1,
			DialingNumServers:   1,
			DialingNumClients:   1,
		},
		false)
	dialingCluster := dialing.Cluster

	require.NoError(t, dialingCluster.ConfigEntryWrite(&api.ProxyConfigEntry{
		Kind: api.ProxyDefaults,
		Name: "global",
		Config: map[string]interface{}{
			"protocol": "http",
		},
	}))

	clientConnectProxy, err := upgrade.CreateAndRegisterStaticClientSidecarWith2Upstreams(dialingCluster,
		[]string{libservice.StaticServerServiceName, "peer-static-server"}, true,
	)
	require.NoErrorf(t, err, "error creating client connect proxy in cluster %s", dialingCluster.NetworkName)

	// make a resolver for service peer-static-server
	resolverConfigEntry := &api.ServiceResolverConfigEntry{
		Kind: api.ServiceResolver,
		Name: "peer-static-server",
		Redirect: &api.ServiceResolverRedirect{
			Service: libservice.StaticServerServiceName,
			Peer:    libtopology.DialingPeerName,
		},
	}
	require.NoErrorf(t, dialingCluster.ConfigEntryWrite(resolverConfigEntry),
		"error writing resolver config entry for %s", resolverConfigEntry.Name)

	// make a resolver for service static-server
	resolverConfigEntry = &api.ServiceResolverConfigEntry{
		Kind: api.ServiceResolver,
		Name: libservice.StaticServerServiceName,
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
	require.NoErrorf(t, dialingCluster.ConfigEntryWrite(resolverConfigEntry),
		"error writing resolver config entry for %s", resolverConfigEntry.Name)

	// Make a static-server in dialing cluster
	serviceOpts := &libservice.ServiceOpts{
		Name:     libservice.StaticServerServiceName,
		ID:       "static-server-dialing",
		HTTPPort: 8081,
		GRPCPort: 8078,
	}
	_, serverConnectProxy, err := libservice.CreateAndRegisterStaticServerAndSidecar(dialingCluster.Clients()[0], serviceOpts)
	require.NoError(t, err)
	libassert.CatalogServiceExists(t, dialingCluster.Clients()[0].GetClient(), libservice.StaticServerServiceName, nil)

	_, appPorts := clientConnectProxy.GetAddrs()

	assertionAdditionalResources := func() {
		// assert traffic can fail-over to static-server in peered cluster and restor to local static-server
		// timeouts in this segment of the test reflect previously implicit retries in fortio name assertions for parity
		libassert.WaitForFortioName(t, &retry.Timer{Timeout: 2 * time.Minute, Wait: 500 * time.Millisecond}, fmt.Sprintf("http://localhost:%d", appPorts[0]), "static-server-dialing", "")
		require.NoError(t, serverConnectProxy.Stop())
		libassert.WaitForFortioName(t, &retry.Timer{Timeout: 2 * time.Minute, Wait: 500 * time.Millisecond}, fmt.Sprintf("http://localhost:%d", appPorts[0]), libservice.StaticServerServiceName, "")
		require.NoError(t, serverConnectProxy.Start())
		libassert.WaitForFortioName(t, &retry.Timer{Timeout: 2 * time.Minute, Wait: 500 * time.Millisecond}, fmt.Sprintf("http://localhost:%d", appPorts[0]), "static-server-dialing", "")

		// assert peer-static-server resolves to static-server in peered cluster
		libassert.AssertFortioName(t, fmt.Sprintf("http://localhost:%d", appPorts[1]), libservice.StaticServerServiceName, "")
	}
	assertionAdditionalResources()

	peeringUpgrade(t, accepting, dialing, utils.TargetVersion)

	require.NoError(t, clientConnectProxy.Restart())
	assertionAdditionalResources()

	peeringPostUpgradeValidation(t, dialing)
	// TODO: restart static-server-2's sidecar
}

// In addtional to the basic topology, this case provisions the following
// services in the dialing cluster:
//
//   - a new static-client at server_0 that has two upstreams: split-static-server (5000)
//     and peer-static-server (5001)
//   - a local static-server service at client_0
//   - service-splitter named split-static-server w/ 2 services: "local-static-server" and
//     "peer-static-server".
//   - service-resolved named local-static-server
//   - service-resolved named peer-static-server
func TestPeering_HTTPResolverAndSplitter(t *testing.T) {
	t.Parallel()

	accepting, dialing := libtopology.BasicPeeringTwoClustersSetup(t, utils.GetLatestImageName(), utils.LatestVersion,
		libtopology.PeeringClusterSize{
			AcceptingNumServers: 1,
			AcceptingNumClients: 1,
			DialingNumServers:   1,
			DialingNumClients:   1,
		},
		false)
	dialingCluster := dialing.Cluster

	require.NoError(t, dialingCluster.ConfigEntryWrite(&api.ProxyConfigEntry{
		Kind: api.ProxyDefaults,
		Name: "global",
		Config: map[string]interface{}{
			"protocol": "http",
		},
	}))

	clientConnectProxy, err := upgrade.CreateAndRegisterStaticClientSidecarWith2Upstreams(dialingCluster,
		[]string{"split-static-server", "peer-static-server"}, true,
	)
	require.NoErrorf(t, err, "creating client connect proxy in cluster %s", dialingCluster.NetworkName)

	// make a resolver for service peer-static-server
	resolverConfigEntry := &api.ServiceResolverConfigEntry{
		Kind: api.ServiceResolver,
		Name: "peer-static-server",
		Redirect: &api.ServiceResolverRedirect{
			Service: libservice.StaticServerServiceName,
			Peer:    libtopology.DialingPeerName,
		},
	}
	require.NoErrorf(t, dialingCluster.ConfigEntryWrite(resolverConfigEntry),
		"writing resolver config entry for %s", resolverConfigEntry.Name)

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
	require.NoErrorf(t, dialingCluster.ConfigEntryWrite(splitter),
		"error writing splitter config entry for %s", splitter.Name)

	// make a resolver for service local-static-server
	resolverConfigEntry = &api.ServiceResolverConfigEntry{
		Kind: api.ServiceResolver,
		Name: "local-static-server",
		Redirect: &api.ServiceResolverRedirect{
			Service: libservice.StaticServerServiceName,
		},
	}
	require.NoErrorf(t, dialingCluster.ConfigEntryWrite(resolverConfigEntry),
		"error writing resolver config entry for %s", resolverConfigEntry.Name)

	// Make a static-server in dialing cluster
	serviceOpts := &libservice.ServiceOpts{
		Name:     libservice.StaticServerServiceName,
		ID:       "static-server",
		HTTPPort: 8081,
		GRPCPort: 8078,
	}
	_, _, err = libservice.CreateAndRegisterStaticServerAndSidecar(dialingCluster.Clients()[0], serviceOpts)
	require.NoError(t, err)
	libassert.CatalogServiceExists(t, dialingCluster.Clients()[0].GetClient(), libservice.StaticServerServiceName, nil)

	_, appPorts := clientConnectProxy.GetAddrs()
	assertionAdditionalResources := func() {
		libassert.HTTPServiceEchoesResHeader(t, "localhost", appPorts[0], "", map[string]string{
			"X-Test-Split": "local",
		})
		libassert.HTTPServiceEchoesResHeader(t, "localhost", appPorts[0], "", map[string]string{
			"X-Test-Split": "peer",
		})
		libassert.HTTPServiceEchoes(t, "localhost", appPorts[0], "")
	}
	assertionAdditionalResources()

	peeringUpgrade(t, accepting, dialing, utils.TargetVersion)

	require.NoError(t, clientConnectProxy.Restart())
	assertionAdditionalResources()

	peeringPostUpgradeValidation(t, dialing)
	// TODO: restart static-server-2's sidecar
}

func peeringUpgrade(t *testing.T, accepting, dialing *libtopology.BuiltCluster, targetVersion string) {
	t.Helper()

	dialingClient, err := dialing.Cluster.GetClient(nil, false)
	require.NoError(t, err)

	acceptingClient, err := accepting.Cluster.GetClient(nil, false)
	require.NoError(t, err)

	_, gatewayAdminPort := dialing.Gateway.GetAdminAddr()
	_, staticClientPort := dialing.Container.GetAddr()

	// Upgrade the accepting cluster and assert peering is still ACTIVE
	require.NoError(t, accepting.Cluster.StandardUpgrade(t, context.Background(), utils.GetTargetImageName(), targetVersion))
	libassert.PeeringStatus(t, acceptingClient, libtopology.AcceptingPeerName, api.PeeringStateActive)
	libassert.PeeringStatus(t, dialingClient, libtopology.DialingPeerName, api.PeeringStateActive)

	require.NoError(t, dialing.Cluster.StandardUpgrade(t, context.Background(), utils.GetTargetImageName(), targetVersion))
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
}

func peeringPostUpgradeValidation(t *testing.T, dialing *libtopology.BuiltCluster) {
	t.Helper()

	clientSidecarService, err := libservice.CreateAndRegisterStaticClientSidecar(dialing.Cluster.Servers()[0], libtopology.DialingPeerName, true, false, nil)
	require.NoError(t, err)
	_, port := clientSidecarService.GetAddr()
	_, adminPort := clientSidecarService.GetAdminAddr()
	libassert.AssertUpstreamEndpointStatus(t, adminPort, fmt.Sprintf("static-server.default.%s.external", libtopology.DialingPeerName), "HEALTHY", 1)
	libassert.HTTPServiceEchoes(t, "localhost", port, "")
	libassert.AssertFortioName(t, fmt.Sprintf("http://localhost:%d", port), libservice.StaticServerServiceName, "")
}
