// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package upgrade

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/go-version"

	"github.com/hashicorp/consul/api"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

// Test health check GRPC call using Target Servers and Latest GA Clients
// Note: this upgrade test doesn't use StandardUpgrade since it requires
// a cluster with clients and servers with mixed versions
func TestTargetServersWithLatestGAClients(t *testing.T) {
	t.Parallel()

	fromVersion, err := version.NewVersion(utils.LatestVersion)
	require.NoError(t, err)
	if fromVersion.LessThan(utils.Version_1_14) {
		t.Skip("TODO: why are we skipping this?")
	}

	const (
		numServers = 3
		numClients = 1
	)

	clusterConfig := &libtopology.ClusterConfig{
		NumServers: numServers,
		NumClients: numClients,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:    "dc1",
			ConsulVersion: utils.TargetVersion,
		},
		ApplyDefaultProxySettings: true,
	}

	cluster, _, _ := libtopology.NewCluster(t, clusterConfig)

	// change the version of Client agent to latest version
	config := cluster.Agents[3].GetConfig()
	config.Version = utils.LatestVersion
	cluster.Agents[3].Upgrade(context.Background(), config)

	client := cluster.APIClient(0)

	libcluster.WaitForLeader(t, cluster, client)
	libcluster.WaitForMembers(t, client, 4)

	const serviceName = "api"
	index := libservice.ServiceCreate(t, client, serviceName)

	require.NoError(t, client.Agent().ServiceRegister(
		&api.AgentServiceRegistration{Name: serviceName, Port: 9998},
	))

	checkServiceHealth(t, client, "api", index)
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
			ConsulImageName: utils.GetLatestImageName(),
			ConsulVersion:   utils.LatestVersion,
		}
		targetOpts = libcluster.BuildOptions{
			ConsulImageName: utils.GetTargetImageName(),
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

	libservice.ClientsCreate(t, numClients, utils.GetLatestImageName(), utils.LatestVersion, cluster)

	client := cluster.APIClient(0)

	libcluster.WaitForLeader(t, cluster, client)
	libcluster.WaitForMembers(t, client, 4) // TODO(rb): why 4?

	const serviceName = "api"
	index := libservice.ServiceCreate(t, client, serviceName)
	require.NoError(t, client.Agent().ServiceRegister(
		&api.AgentServiceRegistration{Name: serviceName, Port: 9998},
	))
	checkServiceHealth(t, client, "api", index)
}
