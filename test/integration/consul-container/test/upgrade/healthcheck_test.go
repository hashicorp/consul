package upgrade

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

// Test health check GRPC call using Target Servers and Latest GA Clients
func TestTargetServersWithLatestGAClients(t *testing.T) {
	t.Parallel()

	const (
		numServers = 3
		numClients = 1
	)

	cluster := serversCluster(t, numServers, utils.TargetImageName, utils.TargetVersion)

	clientsCreate(t, numClients, utils.LatestImageName, utils.LatestVersion, cluster)

	client := cluster.APIClient(0)

	libcluster.WaitForLeader(t, cluster, client)
	libcluster.WaitForMembers(t, client, 4)

	const serviceName = "api"
	index := serviceCreate(t, client, serviceName)

	ch, errCh := serviceHealthBlockingQuery(client, serviceName, index)
	require.NoError(t, client.Agent().ServiceRegister(
		&api.AgentServiceRegistration{Name: serviceName, Port: 9998},
	))

	timer := time.NewTimer(3 * time.Second)
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case service := <-ch:
		require.Len(t, service, 1)
		require.Equal(t, serviceName, service[0].Service.Service)
		require.Equal(t, 9998, service[0].Service.Port)
	case <-timer.C:
		t.Fatalf("test timeout")
	}
}

// Test health check GRPC call using Mixed (majority latest) Servers and Latest GA Clients
func TestMixedServersMajorityLatestGAClient(t *testing.T) {
	t.Parallel()

	testMixedServersGAClient(t, false)
}

// Test health check GRPC call using Mixed (majority target) Servers and Latest GA Clients
func TestMixedServersMajorityTargetGAClient(t *testing.T) {
	t.Parallel()

	testMixedServersGAClient(t, true)
}

// Test health check GRPC call using Mixed (majority conditional) Servers and Latest GA Clients
func testMixedServersGAClient(t *testing.T, majorityIsTarget bool) {
	var (
		latestOpts = libcluster.BuildOptions{
			ConsulImageName: utils.LatestImageName,
			ConsulVersion:   utils.LatestVersion,
		}
		targetOpts = libcluster.BuildOptions{
			ConsulImageName: utils.TargetImageName,
			ConsulVersion:   utils.TargetVersion,
		}

		majorityOpts libcluster.BuildOptions
		minorityOpts libcluster.BuildOptions
	)

	if majorityIsTarget {
		majorityOpts = targetOpts
		minorityOpts = latestOpts
	} else {
		majorityOpts = latestOpts
		minorityOpts = targetOpts
	}

	const (
		numServers = 3
		numClients = 1
	)

	var configs []libcluster.Config
	{
		ctx := libcluster.NewBuildContext(t, minorityOpts)

		conf := libcluster.NewConfigBuilder(ctx).
			ToAgentConfig(t)
		t.Logf("Cluster server (leader) config:\n%s", conf.JSON)

		configs = append(configs, *conf)
	}

	{
		ctx := libcluster.NewBuildContext(t, majorityOpts)

		conf := libcluster.NewConfigBuilder(ctx).
			Bootstrap(numServers).
			ToAgentConfig(t)
		t.Logf("Cluster server config:\n%s", conf.JSON)

		for i := 1; i < numServers; i++ {
			configs = append(configs, *conf)
		}
	}

	cluster, err := libcluster.New(t, configs)
	require.NoError(t, err)

	clientsCreate(t, numClients, utils.LatestImageName, utils.LatestVersion, cluster)

	client := cluster.APIClient(0)

	libcluster.WaitForLeader(t, cluster, client)
	libcluster.WaitForMembers(t, client, 4) // TODO(rb): why 4?

	const serviceName = "api"
	index := serviceCreate(t, client, serviceName)

	ch, errCh := serviceHealthBlockingQuery(client, serviceName, index)
	require.NoError(t, client.Agent().ServiceRegister(
		&api.AgentServiceRegistration{Name: serviceName, Port: 9998},
	))

	timer := time.NewTimer(3 * time.Second)
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case service := <-ch:
		require.Len(t, service, 1)
		require.Equal(t, serviceName, service[0].Service.Service)
		require.Equal(t, 9998, service[0].Service.Port)
	case <-timer.C:
		t.Fatalf("test timeout")
	}
}
