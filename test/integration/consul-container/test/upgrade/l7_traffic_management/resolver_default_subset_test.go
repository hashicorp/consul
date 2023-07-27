// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package upgrade

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/stretchr/testify/require"
)

// Test resolver directs traffic to default subset
// - Create 2 additional static-server instances: one in V1 subset and the other in V2 subset
// - resolver directs traffic to the default subset, which is V2.
func TestTrafficManagement_ResolverDefaultSubset(t *testing.T) {
	t.Parallel()

	cluster, staticServerProxy, staticClientProxy := setup(t)

	node := cluster.Agents[0]
	client := node.GetClient()

	// Create static-server-v1 and static-server-v2
	serviceOptsV1 := &libservice.ServiceOpts{
		Name:     libservice.StaticServerServiceName,
		ID:       "static-server-v1",
		Meta:     map[string]string{"version": "v1"},
		HTTPPort: 8081,
		GRPCPort: 8078,
	}
	_, serverConnectProxyV1, err := libservice.CreateAndRegisterStaticServerAndSidecar(node, serviceOptsV1)
	require.NoError(t, err)

	serviceOptsV2 := &libservice.ServiceOpts{
		Name:     libservice.StaticServerServiceName,
		ID:       "static-server-v2",
		Meta:     map[string]string{"version": "v2"},
		HTTPPort: 8082,
		GRPCPort: 8077,
	}
	_, serverConnectProxyV2, err := libservice.CreateAndRegisterStaticServerAndSidecar(node, serviceOptsV2)
	require.NoError(t, err)
	libassert.CatalogServiceExists(t, client, libservice.StaticServerServiceName, nil)

	// TODO: verify the number of instance of static-server is 3
	libassert.AssertServiceHasHealthyInstances(t, node, libservice.StaticServerServiceName, true, 3)

	// Register service resolver
	serviceResolver := &api.ServiceResolverConfigEntry{
		Kind:          api.ServiceResolver,
		Name:          libservice.StaticServerServiceName,
		DefaultSubset: "v2",
		Subsets: map[string]api.ServiceResolverSubset{
			"v1": {
				Filter: "Service.Meta.version == v1",
			},
			"v2": {
				Filter: "Service.Meta.version == v2",
			},
		},
	}
	require.NoError(t, cluster.ConfigEntryWrite(serviceResolver))

	assertionFn := func() {
		_, serverAdminPortV1 := serverConnectProxyV1.GetAdminAddr()
		_, serverAdminPortV2 := serverConnectProxyV2.GetAdminAddr()
		_, adminPort := staticClientProxy.GetAdminAddr() // httpPort
		_, port := staticClientProxy.GetAddr()           // EnvoyAdminPort

		libassert.AssertEnvoyRunning(t, serverAdminPortV1)
		libassert.AssertEnvoyRunning(t, serverAdminPortV2)

		libassert.AssertEnvoyPresentsCertURI(t, serverAdminPortV1, libservice.StaticServerServiceName)
		libassert.AssertEnvoyPresentsCertURI(t, serverAdminPortV2, libservice.StaticServerServiceName)

		libassert.AssertUpstreamEndpointStatus(t, adminPort, "v2.static-server.default", "HEALTHY", 1)

		// assert static-server proxies should be healthy
		libassert.AssertServiceHasHealthyInstances(t, node, libservice.StaticServerServiceName, true, 3)

		libassert.AssertUpstreamEndpointStatus(t, adminPort, "v2.static-server.default", "HEALTHY", 1)

		// static-client upstream should connect to static-server-v2 because the default subset value is to v2 set in the service resolver
		libassert.AssertFortioName(t, fmt.Sprintf("http://localhost:%d", port), "static-server-v2", "")
	}

	// validate client and proxy is up and running
	libassert.AssertContainerState(t, staticClientProxy, "running")
	validate(t, staticServerProxy, staticClientProxy)
	assertionFn()

	// Upgrade cluster, restart sidecars then begin service traffic validation
	require.NoError(t, cluster.StandardUpgrade(t, context.Background(), utils.GetTargetImageName(), utils.TargetVersion))
	require.NoError(t, staticClientProxy.Restart())
	require.NoError(t, staticServerProxy.Restart())
	require.NoError(t, serverConnectProxyV1.Restart())
	require.NoError(t, serverConnectProxyV2.Restart())

	validate(t, staticServerProxy, staticClientProxy)
	assertionFn()
}

// Test resolver resolves service instance based on their check status
// - Create one addtional static-server with checks and V1 subset
// - resolver directs traffic to "test" service
func TestTrafficManagement_ResolverDefaultOnlyPassing(t *testing.T) {
	t.Parallel()

	cluster, staticServerProxy, staticClientProxy := setup(t)
	node := cluster.Agents[0]

	serviceOptsV1 := &libservice.ServiceOpts{
		Name:     libservice.StaticServerServiceName,
		ID:       "static-server-v1",
		Meta:     map[string]string{"version": "v1"},
		HTTPPort: 8081,
		GRPCPort: 8078,
		Checks: libservice.Checks{
			Name: "main",
			TTL:  "30m",
		},
		Connect: libservice.SidecarService{
			Port: 21011,
		},
	}
	_, serverConnectProxyV1, err := libservice.CreateAndRegisterStaticServerAndSidecarWithChecks(node, serviceOptsV1)
	require.NoError(t, err)

	// Register service resolver
	serviceResolver := &api.ServiceResolverConfigEntry{
		Kind:          api.ServiceResolver,
		Name:          libservice.StaticServerServiceName,
		DefaultSubset: "test",
		Subsets: map[string]api.ServiceResolverSubset{
			"test": {
				OnlyPassing: true,
			},
		},
		ConnectTimeout: 120 * time.Second,
	}
	_, serverAdminPortV1 := serverConnectProxyV1.GetAdminAddr()

	assertionFn := func() {
		_, port := staticClientProxy.GetAddr()
		_, adminPort := staticClientProxy.GetAdminAddr()
		// force static-server-v1 into a warning state
		err = node.GetClient().Agent().UpdateTTL("service:static-server-v1", "", "warn")
		require.NoError(t, err)

		// ###########################
		// ## with onlypassing=true
		// assert only one static-server proxy is healthy
		err = cluster.ConfigEntryWrite(serviceResolver)
		require.NoError(t, err)
		libassert.AssertServiceHasHealthyInstances(t, node, libservice.StaticServerServiceName, true, 1)

		libassert.AssertEnvoyRunning(t, serverAdminPortV1)
		libassert.AssertEnvoyPresentsCertURI(t, serverAdminPortV1, libservice.StaticServerServiceName)

		// assert static-server proxies should be healthy
		libassert.AssertServiceHasHealthyInstances(t, node, libservice.StaticServerServiceName, true, 1)

		// static-client upstream should have 1 healthy endpoint for test.static-server
		libassert.AssertUpstreamEndpointStatus(t, adminPort, "test.static-server.default", "HEALTHY", 1)

		// static-client upstream should have 1 unhealthy endpoint for test.static-server
		libassert.AssertUpstreamEndpointStatus(t, adminPort, "test.static-server.default", "UNHEALTHY", 1)

		// static-client upstream should connect to static-server since it is passing
		libassert.AssertFortioName(t, fmt.Sprintf("http://localhost:%d", port), libservice.StaticServerServiceName, "")

		// ###########################
		// ## with onlypassing=false
		// revert to OnlyPassing=false by deleting the config
		err = cluster.ConfigEntryDelete(serviceResolver)
		require.NoError(t, err)

		// Consul health check assert only one static-server proxy is healthy when onlyPassing is false
		libassert.AssertServiceHasHealthyInstances(t, node, libservice.StaticServerServiceName, false, 2)

		// Although the service status is in warning state, when onlypassing is set to false Envoy
		// health check returns all service instances with "warning" or "passing" state as Healthy enpoints
		libassert.AssertUpstreamEndpointStatus(t, adminPort, "static-server.default", "HEALTHY", 2)

		// static-client upstream should have 0 unhealthy endpoint for static-server
		libassert.AssertUpstreamEndpointStatus(t, adminPort, "static-server.default", "UNHEALTHY", 0)
	}
	// validate client and proxy is up and running
	libassert.AssertContainerState(t, staticClientProxy, "running")
	validate(t, staticServerProxy, staticClientProxy)
	assertionFn()

	// Upgrade cluster, restart sidecars then begin service traffic validation
	require.NoError(t, cluster.StandardUpgrade(t, context.Background(), utils.GetTargetImageName(), utils.TargetVersion))
	require.NoError(t, staticClientProxy.Restart())
	require.NoError(t, staticServerProxy.Restart())
	require.NoError(t, serverConnectProxyV1.Restart())

	validate(t, staticServerProxy, staticClientProxy)
	assertionFn()
}

// Test resolver directs traffic to default subset
// - Create 3 static-server-2 server instances: one in V1, one in V2, one without any version
// - service2Resolver directs traffic to static-server-2-v2V
func TestTrafficManagement_ResolverSubsetRedirect(t *testing.T) {
	t.Parallel()

	cluster, staticServerProxy, staticClientProxy := setup(t)

	node := cluster.Agents[0]
	client := node.GetClient()

	serviceOpts2 := &libservice.ServiceOpts{
		Name:     libservice.StaticServer2ServiceName,
		ID:       "static-server-2",
		HTTPPort: 8081,
		GRPCPort: 8078,
	}
	_, server2ConnectProxy, err := libservice.CreateAndRegisterStaticServerAndSidecar(node, serviceOpts2)
	require.NoError(t, err)
	libassert.CatalogServiceExists(t, client, libservice.StaticServer2ServiceName, nil)

	serviceOptsV1 := &libservice.ServiceOpts{
		Name:     libservice.StaticServer2ServiceName,
		ID:       "static-server-2-v1",
		Meta:     map[string]string{"version": "v1"},
		HTTPPort: 8082,
		GRPCPort: 8077,
	}
	_, server2ConnectProxyV1, err := libservice.CreateAndRegisterStaticServerAndSidecar(node, serviceOptsV1)
	require.NoError(t, err)

	serviceOptsV2 := &libservice.ServiceOpts{
		Name:     libservice.StaticServer2ServiceName,
		ID:       "static-server-2-v2",
		Meta:     map[string]string{"version": "v2"},
		HTTPPort: 8083,
		GRPCPort: 8076,
	}
	_, server2ConnectProxyV2, err := libservice.CreateAndRegisterStaticServerAndSidecar(node, serviceOptsV2)
	require.NoError(t, err)
	libassert.CatalogServiceExists(t, client, libservice.StaticServer2ServiceName, nil)

	// Register static-server service resolver
	serviceResolver := &api.ServiceResolverConfigEntry{
		Kind: api.ServiceResolver,
		Name: libservice.StaticServer2ServiceName,
		Subsets: map[string]api.ServiceResolverSubset{
			"v1": {
				Filter: "Service.Meta.version == v1",
			},
			"v2": {
				Filter: "Service.Meta.version == v2",
			},
		},
	}
	require.NoError(t, cluster.ConfigEntryWrite(serviceResolver))

	// Register static-server-2 service resolver to redirect traffic
	// from static-server to static-server-2-v2
	server2Resolver := &api.ServiceResolverConfigEntry{
		Kind: api.ServiceResolver,
		Name: libservice.StaticServerServiceName,
		Redirect: &api.ServiceResolverRedirect{
			Service:       libservice.StaticServer2ServiceName,
			ServiceSubset: "v2",
		},
	}
	require.NoError(t, cluster.ConfigEntryWrite(server2Resolver))

	assertionFn := func() {
		_, server2AdminPort := server2ConnectProxy.GetAdminAddr()
		_, server2AdminPortV1 := server2ConnectProxyV1.GetAdminAddr()
		_, server2AdminPortV2 := server2ConnectProxyV2.GetAdminAddr()

		// assert 3 static-server-2 instances are healthy
		libassert.AssertServiceHasHealthyInstances(t, node, libservice.StaticServer2ServiceName, false, 3)

		libassert.AssertEnvoyRunning(t, server2AdminPort)
		libassert.AssertEnvoyRunning(t, server2AdminPortV1)
		libassert.AssertEnvoyRunning(t, server2AdminPortV2)

		libassert.AssertEnvoyPresentsCertURI(t, server2AdminPort, libservice.StaticServer2ServiceName)
		libassert.AssertEnvoyPresentsCertURI(t, server2AdminPortV1, libservice.StaticServer2ServiceName)
		libassert.AssertEnvoyPresentsCertURI(t, server2AdminPortV2, libservice.StaticServer2ServiceName)

		// assert static-server proxies should be healthy
		libassert.AssertServiceHasHealthyInstances(t, node, libservice.StaticServer2ServiceName, true, 3)

		_, appPort := staticClientProxy.GetAddr()
		_, adminPort := staticClientProxy.GetAdminAddr()

		libassert.AssertFortioName(t, fmt.Sprintf("http://localhost:%d", appPort), "static-server-2-v2", "")
		libassert.AssertUpstreamEndpointStatus(t, adminPort, "v2.static-server-2.default", "HEALTHY", 1)

	}
	require.NoError(t, err)
	// validate client and proxy is up and running
	libassert.AssertContainerState(t, staticClientProxy, "running")
	validate(t, staticServerProxy, staticClientProxy)
	assertionFn()

	// Upgrade cluster, restart sidecars then begin service traffic validation
	require.NoError(t, cluster.StandardUpgrade(t, context.Background(), utils.GetTargetImageName(), utils.TargetVersion))
	require.NoError(t, staticClientProxy.Restart())
	require.NoError(t, staticServerProxy.Restart())
	require.NoErrorf(t, server2ConnectProxy.Restart(), "%s", server2ConnectProxy.GetName())
	require.NoErrorf(t, server2ConnectProxyV1.Restart(), "%s", server2ConnectProxyV1.GetName())
	require.NoErrorf(t, server2ConnectProxyV2.Restart(), "%s", server2ConnectProxyV2.GetName())

	validate(t, staticServerProxy, staticClientProxy)
	assertionFn()
}

func setup(t *testing.T) (*libcluster.Cluster, libservice.Service, libservice.Service) {
	buildOpts := &libcluster.BuildOptions{
		ConsulImageName:      utils.GetLatestImageName(),
		ConsulVersion:        utils.LatestVersion,
		Datacenter:           "dc1",
		InjectAutoEncryption: true,
	}

	cluster, _, _ := topology.NewCluster(t, &topology.ClusterConfig{
		NumServers:                1,
		NumClients:                1,
		BuildOpts:                 buildOpts,
		ApplyDefaultProxySettings: true,
	})
	node := cluster.Agents[0]
	client := node.GetClient()

	serviceOpts := &libservice.ServiceOpts{
		Name:     libservice.StaticServerServiceName,
		ID:       "static-server",
		HTTPPort: 8080,
		GRPCPort: 8079,
	}
	_, staticServerProxy, err := libservice.CreateAndRegisterStaticServerAndSidecar(node, serviceOpts)
	require.NoError(t, err)

	// Create a client proxy instance with the server as an upstream
	staticClientProxy, err := libservice.CreateAndRegisterStaticClientSidecar(node, "", false, false)
	require.NoError(t, err)

	require.NoError(t, err)
	libassert.CatalogServiceExists(t, client, libservice.StaticServerServiceName, nil)
	libassert.CatalogServiceExists(t, client, fmt.Sprintf("%s-sidecar-proxy", libservice.StaticClientServiceName), nil)

	err = cluster.ConfigEntryWrite(&api.ProxyConfigEntry{
		Kind: api.ProxyDefaults,
		Name: "global",
		Config: map[string]interface{}{
			"protocol": "http",
		},
	})
	require.NoError(t, err)

	validate(t, staticServerProxy, staticClientProxy)

	return cluster, staticServerProxy, staticClientProxy
}

func validate(t *testing.T, staticServerProxy, staticClientProxy libservice.Service) {
	_, port := staticClientProxy.GetAddr()
	_, adminPort := staticClientProxy.GetAdminAddr()
	_, serverAdminPort := staticServerProxy.GetAdminAddr()
	libassert.HTTPServiceEchoes(t, "localhost", port, "")
	libassert.AssertEnvoyRunning(t, adminPort)
	libassert.AssertEnvoyRunning(t, serverAdminPort)
	libassert.AssertEnvoyPresentsCertURI(t, adminPort, libservice.StaticClientServiceName)
	libassert.AssertEnvoyPresentsCertURI(t, serverAdminPort, libservice.StaticServerServiceName)
}
