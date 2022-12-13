package cluster

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	// "github.com/hashicorp/consul/sdk/testutil/retry"
	libagent "github.com/hashicorp/consul/test/integration/consul-container/libs/agent"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

// creatingAcceptingClusterAndSetup creates a cluster with 3 servers and 1 client.
// It also creates and registers a service+sidecar.
// The API client returned is pointed at the client agent.
func CreatingAcceptingClusterAndSetup(t *testing.T, numServer int, version string, acceptingPeerName string) (*Cluster, *api.Client, *libagent.BuildContext) {
	var configs []libagent.Config

	opts := libagent.BuildOptions{
		InjectAutoEncryption:   true,
		InjectGossipEncryption: true,
		ConsulVersion:          version,
	}
	ctx, err := libagent.NewBuildContext(opts)
	require.NoError(t, err)

	for i := 0; i < numServer; i++ {
		serverConf, err := libagent.NewConfigBuilder(ctx).
			Bootstrap(numServer).
			Peering(true).
			RetryJoin(fmt.Sprintf("agent-%d", (i+1)%3)). // Round-robin join the servers
			ToAgentConfig()
		require.NoError(t, err)
		t.Logf("dc1 server config %d: \n%s", i, serverConf.JSON)

		configs = append(configs, *serverConf)
	}

	// Add a stable client to register the service
	clientConf, err := libagent.NewConfigBuilder(ctx).
		Client().
		Peering(true).
		RetryJoin("agent-0", "agent-1", "agent-2").
		ToAgentConfig()
	require.NoError(t, err)

	t.Logf("dc1 client config: \n%s", clientConf.JSON)

	configs = append(configs, *clientConf)

	cluster, err := New(configs)
	require.NoError(t, err)

	// Use the client agent as the HTTP endpoint since we will not rotate it
	clientNode := cluster.Agents[numServer]
	client := clientNode.GetClient()
	WaitForLeader(t, cluster, client)
	WaitForMembers(t, client, numServer+1)

	// Default Proxy Settings
	ok, err := utils.ApplyDefaultProxySettings(client)
	require.NoError(t, err)
	require.True(t, ok)

	// Create the mesh gateway for dataplane traffic
	_, err = libservice.NewGatewayService(context.Background(), "mesh", "mesh", clientNode)
	require.NoError(t, err)

	// Create a service and proxy instance
	_, _, err = libservice.CreateAndRegisterStaticServerAndSidecar(clientNode)
	require.NoError(t, err)

	libassert.CatalogServiceExists(t, client, "static-server")
	libassert.CatalogServiceExists(t, client, "static-server-sidecar-proxy")

	// Export the service
	config := &api.ExportedServicesConfigEntry{
		Name: "default",
		Services: []api.ExportedService{
			{
				Name: "static-server",
				Consumers: []api.ServiceConsumer{
					// TODO: need to handle the changed field name in 1.13
					{Peer: acceptingPeerName},
				},
			},
		},
	}

	ok, _, err = client.ConfigEntries().Set(config, &api.WriteOptions{})
	require.NoError(t, err)
	require.True(t, ok)

	return cluster, client, ctx
}

// createDialingClusterAndSetup creates a cluster for peering with a single dev agent
func CreateDialingClusterAndSetup(t *testing.T, version string, dialingPeerName string) (*Cluster, *api.Client, libservice.Service) {
	opts := libagent.BuildOptions{
		Datacenter:             "dc2",
		InjectAutoEncryption:   true,
		InjectGossipEncryption: true,
		ConsulVersion:          version,
	}
	ctx, err := libagent.NewBuildContext(opts)
	require.NoError(t, err)

	conf, err := libagent.NewConfigBuilder(ctx).
		Peering(true).
		ToAgentConfig()
	require.NoError(t, err)
	t.Logf("dc2 server config: \n%s", conf.JSON)

	configs := []libagent.Config{*conf}

	cluster, err := New(configs)
	require.NoError(t, err)

	node := cluster.Agents[0]
	client := node.GetClient()
	WaitForLeader(t, cluster, client)
	WaitForMembers(t, client, 1)

	// Default Proxy Settings
	ok, err := utils.ApplyDefaultProxySettings(client)
	require.NoError(t, err)
	require.True(t, ok)

	// Create the mesh gateway for dataplane traffic
	_, err = libservice.NewGatewayService(context.Background(), "mesh", "mesh", node)
	require.NoError(t, err)

	// Create a service and proxy instance
	clientProxyService, err := libservice.CreateAndRegisterStaticClientSidecar(node, dialingPeerName, true)
	require.NoError(t, err)

	libassert.CatalogServiceExists(t, client, "static-client-sidecar-proxy")

	return cluster, client, clientProxyService
}
