package topology

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

// BasicSingleClusterTopology sets up a scenario for single cluster test.
func BasicSingleClusterTopology(t *testing.T, clusterOpts *libcluster.Options) *libcluster.Cluster {
	opts := libcluster.BuildOptions{
		Datacenter:             clusterOpts.Datacenter,
		InjectAutoEncryption:   true,
		InjectGossipEncryption: true,
		ConsulVersion:          clusterOpts.Version,
	}
	ctx, err := libcluster.NewBuildContext(opts)
	require.NoError(t, err)

	var configs []libcluster.Config
	numServer := clusterOpts.NumServer
	for i := 0; i < numServer; i++ {
		serverConf, err := libcluster.NewConfigBuilder(ctx).
			Bootstrap(numServer).
			Peering(true).
			Telemetry("127.0.0.0:2180").
			RetryJoin(fmt.Sprintf("agent-%d", (i+1)%numServer)). // Round-robin join the servers
			ToAgentConfig()
		require.NoError(t, err)
		t.Logf("%s server config %d: \n%s", clusterOpts.Datacenter, i, serverConf.JSON)

		configs = append(configs, *serverConf)
	}

	// Add a stable client to register the service
	clientConf, err := libcluster.NewConfigBuilder(ctx).
		Client().
		Peering(true).
		RetryJoin("agent-0", "agent-1", "agent-2").
		ToAgentConfig()
	require.NoError(t, err)
	t.Logf("%s client config: \n%s", clusterOpts.Datacenter, clientConf.JSON)
	configs = append(configs, *clientConf)

	cluster, err := libcluster.New(configs)
	require.NoError(t, err)
	cluster.BuildContext = ctx

	// node := cluster.Agents[0]
	// Use the client agent as the HTTP endpoint since we will not rotate it
	clientNodes, err := cluster.Clients()
	require.NoError(t, err)
	require.True(t, len(clientNodes) > 0)
	client := clientNodes[0].GetClient()
	libcluster.WaitForLeader(t, cluster, client)
	libcluster.WaitForMembers(t, client, numServer+1)

	// Default Proxy Settings
	ok, err := utils.ApplyDefaultProxySettings(client)
	require.NoError(t, err)
	require.True(t, ok)

	return cluster
}
