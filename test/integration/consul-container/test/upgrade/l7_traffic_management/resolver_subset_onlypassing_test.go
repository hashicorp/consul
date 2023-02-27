package upgrade

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	libutils "github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTrafficManagement_ServiceResolverSubsetOnlyPassing Summary
// This test starts up 2 servers and 1 client in the same datacenter.
//
// Steps:
//   - Create a single agent cluster.
//   - Create one static-server, 1 subset server 1 client and sidecars for all services, then register them with Consul
func TestTrafficManagement_ServiceResolverSubsetOnlyPassing(t *testing.T) {
	t.Parallel()

	responseFormat := map[string]string{"format": "json"}

	type testcase struct {
		oldversion    string
		targetVersion string
	}
	tcs := []testcase{
		{
			oldversion:    "1.13",
			targetVersion: utils.TargetVersion,
		},
		{
			oldversion:    "1.14",
			targetVersion: utils.TargetVersion,
		},
	}

	run := func(t *testing.T, tc testcase) {
		buildOpts := &libcluster.BuildOptions{
			ConsulVersion:        tc.oldversion,
			Datacenter:           "dc1",
			InjectAutoEncryption: true,
		}
		// If version < 1.14 disable AutoEncryption
		oldVersion, _ := version.NewVersion(tc.oldversion)
		if oldVersion.LessThan(libutils.Version_1_14) {
			buildOpts.InjectAutoEncryption = false
		}
		cluster, _, _ := topology.NewPeeringCluster(t, 1, buildOpts)
		node := cluster.Agents[0]

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
		err := cluster.ConfigEntryWrite(serviceResolver)
		require.NoError(t, err)

		serverConnectProxy, serverConnectProxyV1, clientConnectProxy := createServiceAndSubset(t, cluster)

		_, port := clientConnectProxy.GetAddr()
		_, adminPort := clientConnectProxy.GetAdminAddr()
		_, serverAdminPort := serverConnectProxy.GetAdminAddr()
		_, serverAdminPortV1 := serverConnectProxyV1.GetAdminAddr()

		// Upgrade cluster, restart sidecars then begin service traffic validation
		require.NoError(t, cluster.StandardUpgrade(t, context.Background(), tc.targetVersion))
		require.NoError(t, clientConnectProxy.Restart())
		require.NoError(t, serverConnectProxy.Restart())
		require.NoError(t, serverConnectProxyV1.Restart())

		// force static-server-v1 into a warning state
		err = node.GetClient().Agent().UpdateTTL("service:static-server-v1", "", "warn")
		assert.NoError(t, err)

		// validate static-client is up and running
		libassert.AssertContainerState(t, clientConnectProxy, "running")
		libassert.HTTPServiceEchoes(t, "localhost", port, "")

		// validate static-client proxy admin is up
		_, clientStatusCode, err := libassert.GetEnvoyOutput(adminPort, "stats", responseFormat)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, clientStatusCode, fmt.Sprintf("service cannot be reached %v", clientStatusCode))

		// validate static-server proxy admin is up
		_, serverStatusCode, err := libassert.GetEnvoyOutput(serverAdminPort, "stats", responseFormat)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, serverStatusCode, fmt.Sprintf("service cannot be reached %v", serverStatusCode))

		// validate static-server-v1 proxy admin is up
		_, serverStatusCodeV1, err := libassert.GetEnvoyOutput(serverAdminPortV1, "stats", responseFormat)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, serverStatusCodeV1, fmt.Sprintf("service cannot be reached %v", serverStatusCodeV1))

		// certs are valid
		libassert.AssertEnvoyPresentsCertURI(t, adminPort, libservice.StaticClientServiceName)
		libassert.AssertEnvoyPresentsCertURI(t, serverAdminPort, libservice.StaticServerServiceName)
		libassert.AssertEnvoyPresentsCertURI(t, serverAdminPortV1, libservice.StaticServerServiceName)

		// ###########################
		// ## with onlypassing=true
		// assert only one static-server proxy is healthy
		libassert.AssertServiceHasHealthyInstances(t, node, libservice.StaticServerServiceName, true, 1)

		// static-client upstream should have 1 healthy endpoint for test.static-server
		libassert.AssertUpstreamEndpointStatus(t, adminPort, "test.static-server.default", "HEALTHY", 1)

		// static-client upstream should have 1 unhealthy endpoint for test.static-server
		libassert.AssertUpstreamEndpointStatus(t, adminPort, "test.static-server.default", "UNHEALTHY", 1)

		// static-client upstream should connect to static-server-v2 because the default subset value is to v2 set in the service resolver
		libassert.AssertFortioName(t, fmt.Sprintf("http://localhost:%d", port), libservice.StaticServerServiceName)

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

	for _, tc := range tcs {
		t.Run(fmt.Sprintf("upgrade from %s to %s", tc.oldversion, tc.targetVersion),
			func(t *testing.T) {
				run(t, tc)
			})
	}
}

// create 2 servers and 1 client
func createServiceAndSubset(t *testing.T, cluster *libcluster.Cluster) (libservice.Service, libservice.Service, libservice.Service) {
	node := cluster.Agents[0]
	client := node.GetClient()

	serviceOpts := &libservice.ServiceOpts{
		Name:     libservice.StaticServerServiceName,
		ID:       libservice.StaticServerServiceName,
		HTTPPort: 8080,
		GRPCPort: 8079,
	}
	_, serverConnectProxy, err := libservice.CreateAndRegisterStaticServerAndSidecar(node, serviceOpts)
	require.NoError(t, err)
	libassert.CatalogServiceExists(t, client, libservice.StaticServerServiceName)

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
	libassert.CatalogServiceExists(t, client, libservice.StaticServerServiceName)

	// Create a client proxy instance with the server as an upstream
	clientConnectProxy, err := libservice.CreateAndRegisterStaticClientSidecar(node, "", false)
	require.NoError(t, err)
	libassert.CatalogServiceExists(t, client, fmt.Sprintf("%s-sidecar-proxy", libservice.StaticClientServiceName))

	return serverConnectProxy, serverConnectProxyV1, clientConnectProxy
}
