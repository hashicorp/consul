package basic

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

// TestBasicConnectService Summary
// This test makes sure two services in the same datacenter have connectivity.
// A simulated client (a direct HTTP call) talks to it's upstream proxy through the
//
// Steps:
//   - Create a single agent cluster.
//   - Create the example static-server and sidecar containers, then register them both with Consul
//   - Create an example static-client sidecar, then register both the service and sidecar with Consul
//   - Make sure a call to the client sidecar local bind port returns a response from the upstream, static-server
func TestBasicConnectService(t *testing.T) {
	t.Parallel()
	cluster := createCluster(t)

	clientService := createServices(t, cluster)
	_, port := clientService.GetAddr()
	_, adminPort := clientService.GetAdminAddr()

	libassert.AssertUpstreamEndpointStatus(t, adminPort, "static-server.default", "HEALTHY", 1)
	libassert.GetEnvoyListenerTCPFilters(t, adminPort)

	libassert.AssertContainerState(t, clientService, "running")
	libassert.HTTPServiceEchoes(t, "localhost", port, "")
	libassert.AssertFortioName(t, fmt.Sprintf("http://localhost:%d", port), "static-server", "")
}

func createCluster(t *testing.T) *libcluster.Cluster {
	opts := libcluster.BuildOptions{
		InjectAutoEncryption:   true,
		InjectGossipEncryption: true,
		// TODO: fix the test to not need the service/envoy stack to use :8500
		AllowHTTPAnyway: true,
	}
	ctx := libcluster.NewBuildContext(t, opts)

	conf := libcluster.NewConfigBuilder(ctx).
		ToAgentConfig(t)
	t.Logf("Cluster config:\n%s", conf.JSON)

	configs := []libcluster.Config{*conf}

	cluster, err := libcluster.New(t, configs)
	require.NoError(t, err)

	node := cluster.Agents[0]
	client := node.GetClient()

	libcluster.WaitForLeader(t, cluster, client)
	libcluster.WaitForMembers(t, client, 1)

	// Default Proxy Settings
	ok, err := utils.ApplyDefaultProxySettings(client)
	require.NoError(t, err)
	require.True(t, ok)

	return cluster
}

func createServices(t *testing.T, cluster *libcluster.Cluster) libservice.Service {
	node := cluster.Agents[0]
	client := node.GetClient()
	// Create a service and proxy instance
	serviceOpts := &libservice.ServiceOpts{
		Name:     libservice.StaticServerServiceName,
		ID:       "static-server",
		HTTPPort: 8080,
		GRPCPort: 8079,
	}

	// Create a service and proxy instance
	_, _, err := libservice.CreateAndRegisterStaticServerAndSidecar(node, serviceOpts)
	require.NoError(t, err)

	libassert.CatalogServiceExists(t, client, "static-server-sidecar-proxy", nil)
	libassert.CatalogServiceExists(t, client, libservice.StaticServerServiceName, nil)

	// Create a client proxy instance with the server as an upstream
	clientConnectProxy, err := libservice.CreateAndRegisterStaticClientSidecar(node, "", false)
	require.NoError(t, err)

	libassert.CatalogServiceExists(t, client, "static-client-sidecar-proxy", nil)

	return clientConnectProxy
}
