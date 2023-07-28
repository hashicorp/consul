package basic

import (
	"testing"

	"github.com/stretchr/testify/require"

	libagent "github.com/hashicorp/consul/test/integration/consul-container/libs/agent"
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
//  * Create a single agent cluster.
//	* Create the example static-server and sidecar containers, then register them both with Consul
//  * Create an example static-client sidecar, then register both the service and sidecar with Consul
//  * Make sure a call to the client sidecar local bind port returns a response from the upstream, static-server
func TestBasicConnectService(t *testing.T) {
	cluster := createCluster(t)
	defer terminate(t, cluster)

	clientService := createServices(t, cluster)
	_, port := clientService.GetAddr()

	libassert.HTTPServiceEchoes(t, "localhost", port)
}

func terminate(t *testing.T, cluster *libcluster.Cluster) {
	err := cluster.Terminate()
	require.NoError(t, err)
}

// createCluster
func createCluster(t *testing.T) *libcluster.Cluster {
	opts := libagent.BuildOptions{
		InjectAutoEncryption:   true,
		InjectGossipEncryption: true,
	}
	ctx, err := libagent.NewBuildContext(opts)
	require.NoError(t, err)

	conf, err := libagent.NewConfigBuilder(ctx).ToAgentConfig()
	require.NoError(t, err)
	t.Logf("Cluster config:\n%s", conf.JSON)

	configs := []libagent.Config{*conf}

	cluster, err := libcluster.New(configs)
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
	_, _, err := libservice.CreateAndRegisterStaticServerAndSidecar(node)
	require.NoError(t, err)

	libassert.CatalogServiceExists(t, client, "static-server-sidecar-proxy")
	libassert.CatalogServiceExists(t, client, "static-server")

	// Create a client proxy instance with the server as an upstream
	clientConnectProxy, err := libservice.CreateAndRegisterStaticClientSidecar(node, "", false)
	require.NoError(t, err)

	libassert.CatalogServiceExists(t, client, "static-client-sidecar-proxy")

	return clientConnectProxy
}
