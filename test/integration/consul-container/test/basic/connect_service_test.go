package basic

import (
	"testing"

	"github.com/stretchr/testify/require"

	libassert "github.com/hashicorp/consul/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/integration/consul-container/libs/cluster"
	libnode "github.com/hashicorp/consul/integration/consul-container/libs/node"
	libservice "github.com/hashicorp/consul/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/integration/consul-container/libs/utils"
)

// TestBasicConnectService Summary
// This test makes sure two services in the same datacenter have connectivity.
// A simulated client (a direct HTTP call) talks to it's upstream proxy through the
//
// Steps:
//  * Create a single node cluster.
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
	configs := []libnode.Config{
		{
			HCL: `ports {
					  dns = 8600
					  http = 8500
					  https = 8501
					  grpc = 8502
					  grpc_tls = 8503
					  serf_lan = 8301
					  serf_wan = 8302
					  server = 8300
					}
					bind_addr = "0.0.0.0"
					advertise_addr = "{{ GetInterfaceIP \"eth0\" }}"
					log_level="DEBUG"
					server=true
					bootstrap = true
					connect {
					  enabled = true
					}`,
			Cmd:     []string{"agent", "-client=0.0.0.0"},
			Version: *utils.TargetImage,
		},
	}

	cluster, err := libcluster.New(configs)
	require.NoError(t, err)

	node := cluster.Nodes[0]
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
	node := cluster.Nodes[0]
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
