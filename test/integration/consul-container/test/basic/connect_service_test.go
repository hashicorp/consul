package basic

import (
	"testing"

	"github.com/stretchr/testify/require"

	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/test/topology"
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
	cluster := topology.BasicSingleClusterTopology(t, &libcluster.Options{
		Datacenter: "dc1",
		NumServer:  1,
		NumClient:  1,
	})
	defer func() {
		err := cluster.Terminate()
		require.NoErrorf(t, err, "termining cluster")
	}()

	clientService := createServices(t, cluster)
	_, port := clientService.GetAddr()

	libassert.HTTPServiceEchoes(t, "localhost", port)
}

func createServices(t *testing.T, cluster *libcluster.Cluster) libservice.Service {
	node := cluster.Agents[0]
	client := node.GetClient()

	// Create a service and proxy instance
	_, _, err := libservice.CreateAndRegisterStaticServerAndSidecar(node, false)
	require.NoError(t, err)

	libassert.CatalogServiceExists(t, client, "static-server-sidecar-proxy")
	libassert.CatalogServiceExists(t, client, "static-server")

	// Create a client proxy instance with the server as an upstream
	clientConnectProxy, err := libservice.CreateAndRegisterStaticClientSidecar(node, "", false)
	require.NoError(t, err)

	libassert.CatalogServiceExists(t, client, "static-client-sidecar-proxy")

	return clientConnectProxy
}
