package cluster

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	libagent "github.com/hashicorp/consul/test/integration/consul-container/libs/agent"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

type Options struct {
	Datacenter string
	NumServer  int
	NumClient  int
	Version    string
}

// CreatingPeeringClusterAndSetup creates a cluster with peering enabled
// It also creates and registers a mesh-gateway at the client agent.
// The API client returned is pointed at the client agent.
func CreatingPeeringClusterAndSetup(t *testing.T, clusterOpts *Options) (*Cluster, *api.Client) {
	var configs []libagent.Config

	opts := libagent.BuildOptions{
		Datacenter:             clusterOpts.Datacenter,
		InjectAutoEncryption:   true,
		InjectGossipEncryption: true,
		ConsulVersion:          clusterOpts.Version,
	}
	ctx, err := libagent.NewBuildContext(opts)
	require.NoError(t, err)

	numServer := clusterOpts.NumServer
	for i := 0; i < numServer; i++ {
		serverConf, err := libagent.NewConfigBuilder(ctx).
			Bootstrap(numServer).
			Peering(true).
			RetryJoin(fmt.Sprintf("agent-%d", (i+1)%3)). // Round-robin join the servers
			ToAgentConfig()
		require.NoError(t, err)
		t.Logf("%s server config %d: \n%s", clusterOpts.Datacenter, i, serverConf.JSON)

		configs = append(configs, *serverConf)
	}

	// Add a stable client to register the service
	clientConf, err := libagent.NewConfigBuilder(ctx).
		Client().
		Peering(true).
		RetryJoin("agent-0", "agent-1", "agent-2").
		ToAgentConfig()
	require.NoError(t, err)

	t.Logf("%s client config: \n%s", clusterOpts.Datacenter, clientConf.JSON)

	configs = append(configs, *clientConf)

	cluster, err := New(configs)
	require.NoError(t, err)
	cluster.BuildContext = ctx

	// Use the client agent as the HTTP endpoint since we will not rotate it
	clientNodes, err := cluster.Clients()
	require.NoError(t, err)
	require.True(t, len(clientNodes) > 0)
	clientNode := clientNodes[0]
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
	return cluster, client
}
