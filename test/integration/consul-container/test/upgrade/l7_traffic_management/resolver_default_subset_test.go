package upgrade

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/hashicorp/consul/api"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	libutils "github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"
)

// TestTrafficManagement_ServiceResolverDefaultSubset Summary
// This test starts up 3 servers and 1 client in the same datacenter.
//
// Steps:
//   - Create a single agent cluster.
//   - Create one static-server and 2 subsets and 1 client and sidecar, then register them with Consul
//   - Validate static-server and 2 subsets are and proxy admin endpoint is healthy - 3 instances
//   - Validate static servers proxy listeners should be up and have right certs
func TestTrafficManagement_ServiceResolverDefaultSubset(t *testing.T) {
	t.Parallel()

	var responseFormat = map[string]string{"format": "json"}

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
		err := cluster.ConfigEntryWrite(serviceResolver)
		require.NoError(t, err)

		serverConnectProxy, serverConnectProxyV1, serverConnectProxyV2, clientConnectProxy := createService(t, cluster)

		_, port := clientConnectProxy.GetAddr()
		_, adminPort := clientConnectProxy.GetAdminAddr()
		_, serverAdminPort := serverConnectProxy.GetAdminAddr()
		_, serverAdminPortV1 := serverConnectProxyV1.GetAdminAddr()
		_, serverAdminPortV2 := serverConnectProxyV2.GetAdminAddr()

		// validate client and proxy is up and running
		libassert.AssertContainerState(t, clientConnectProxy, "running")

		libassert.HTTPServiceEchoes(t, "localhost", port, "")
		libassert.AssertUpstreamEndpointStatus(t, adminPort, "v2.static-server.default", "HEALTHY", 1)

		// Upgrade cluster, restart sidecars then begin service traffic validation
		require.NoError(t, cluster.StandardUpgrade(t, context.Background(), tc.targetVersion))
		require.NoError(t, clientConnectProxy.Restart())
		require.NoError(t, serverConnectProxy.Restart())
		require.NoError(t, serverConnectProxyV1.Restart())
		require.NoError(t, serverConnectProxyV2.Restart())

		// POST upgrade validation; repeat client & proxy validation
		libassert.HTTPServiceEchoes(t, "localhost", port, "")
		libassert.AssertUpstreamEndpointStatus(t, adminPort, "v2.static-server.default", "HEALTHY", 1)

		// validate  static-client proxy admin is up
		_, statusCode, err := libassert.GetEnvoyOutput(adminPort, "stats", responseFormat)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, statusCode, fmt.Sprintf("service cannot be reached %v", statusCode))

		// validate static-server proxy admin is up
		_, statusCode1, err := libassert.GetEnvoyOutput(serverAdminPort, "stats", responseFormat)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, statusCode1, fmt.Sprintf("service cannot be reached %v", statusCode1))

		// validate static-server-v1 proxy admin is up
		_, statusCode2, err := libassert.GetEnvoyOutput(serverAdminPortV1, "stats", responseFormat)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, statusCode2, fmt.Sprintf("service cannot be reached %v", statusCode2))

		// validate static-server-v2 proxy admin is up
		_, statusCode3, err := libassert.GetEnvoyOutput(serverAdminPortV2, "stats", responseFormat)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, statusCode3, fmt.Sprintf("service cannot be reached %v", statusCode3))

		// certs are valid
		libassert.AssertEnvoyPresentsCertURI(t, adminPort, "static-client")
		libassert.AssertEnvoyPresentsCertURI(t, serverAdminPort, "static-server")
		libassert.AssertEnvoyPresentsCertURI(t, serverAdminPortV1, "static-server")
		libassert.AssertEnvoyPresentsCertURI(t, serverAdminPortV2, "static-server")

		// assert static-server proxies should be healthy
		libassert.AssertServiceHasHealthyInstances(t, node, libservice.StaticServerServiceName, true, 3)

		// static-client upstream should connect to static-server-v2 because the default subset value is to v2 set in the service resolver
		libassert.AssertFortioName(t, fmt.Sprintf("http://localhost:%d", port), "static-server-v2")
	}

	for _, tc := range tcs {
		t.Run(fmt.Sprintf("upgrade from %s to %s", tc.oldversion, tc.targetVersion),
			func(t *testing.T) {
				run(t, tc)
			})
	}
}

// create 3 servers and 1 client
func createService(t *testing.T, cluster *libcluster.Cluster) (libservice.Service, libservice.Service, libservice.Service, libservice.Service) {
	node := cluster.Agents[0]
	client := node.GetClient()

	serviceOpts := &libservice.ServiceOpts{
		Name:     libservice.StaticServerServiceName,
		ID:       "static-server",
		HTTPPort: 8080,
		GRPCPort: 8079,
	}
	_, serverConnectProxy, err := libservice.CreateAndRegisterStaticServerAndSidecar(node, serviceOpts)
	require.NoError(t, err)
	libassert.CatalogServiceExists(t, client, "static-server")

	serviceOptsV1 := &libservice.ServiceOpts{
		Name:     libservice.StaticServerServiceName,
		ID:       "static-server-v1",
		Meta:     map[string]string{"version": "v1"},
		HTTPPort: 8081,
		GRPCPort: 8078,
	}
	_, serverConnectProxyV1, err := libservice.CreateAndRegisterStaticServerAndSidecar(node, serviceOptsV1)
	require.NoError(t, err)
	libassert.CatalogServiceExists(t, client, "static-server")

	serviceOptsV2 := &libservice.ServiceOpts{
		Name:     libservice.StaticServerServiceName,
		ID:       "static-server-v2",
		Meta:     map[string]string{"version": "v2"},
		HTTPPort: 8082,
		GRPCPort: 8077,
	}
	_, serverConnectProxyV2, err := libservice.CreateAndRegisterStaticServerAndSidecar(node, serviceOptsV2)
	require.NoError(t, err)
	libassert.CatalogServiceExists(t, client, "static-server")

	// Create a client proxy instance with the server as an upstream
	clientConnectProxy, err := libservice.CreateAndRegisterStaticClientSidecar(node, "", false)
	require.NoError(t, err)
	libassert.CatalogServiceExists(t, client, fmt.Sprintf("%s-sidecar-proxy", libservice.StaticClientServiceName))

	return serverConnectProxy, serverConnectProxyV1, serverConnectProxyV2, clientConnectProxy
}
