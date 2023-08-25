// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package upgrade

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

// Test upgrade a cluster of latest version to the target version
func TestBasic(t *testing.T) {
	t.Parallel()

	const numServers = 1
	buildOpts := &libcluster.BuildOptions{
		ConsulImageName:      utils.GetLatestImageName(),
		ConsulVersion:        utils.LatestVersion,
		Datacenter:           "dc1",
		InjectAutoEncryption: true,
	}

	cluster, _, _ := topology.NewCluster(t, &topology.ClusterConfig{
		NumServers:                1,
		BuildOpts:                 buildOpts,
		ApplyDefaultProxySettings: true,
	})

	client := cluster.APIClient(0)

	libcluster.WaitForLeader(t, cluster, client)
	libcluster.WaitForMembers(t, client, numServers)

	// Create a service to be stored in the snapshot
	const serviceName = "api"
	index := libservice.ServiceCreate(t, client, serviceName)

	require.NoError(t, client.Agent().ServiceRegister(
		&api.AgentServiceRegistration{Name: serviceName, Port: 9998},
	))
	checkServiceHealth(t, client, "api", index)

	// upgrade the cluster to the Target version
	t.Logf("initiating standard upgrade to version=%q", utils.TargetVersion)
	err := cluster.StandardUpgrade(t, context.Background(), utils.GetTargetImageName(), utils.TargetVersion)

	require.NoError(t, err)
	libcluster.WaitForLeader(t, cluster, client)
	libcluster.WaitForMembers(t, client, numServers)

	// Verify service is restored from the snapshot
	retry.RunWith(&retry.Timer{Timeout: 5 * time.Second, Wait: 500 * time.Microsecond}, t, func(r *retry.R) {
		service, _, err := client.Catalog().Service(serviceName, "", &api.QueryOptions{})
		require.NoError(r, err)
		require.Len(r, service, 1)
		require.Equal(r, serviceName, service[0].ServiceName)
	})
}

func checkServiceHealth(t *testing.T, client *api.Client, serviceName string, index uint64) {
	failer := func() *retry.Timer {
		return &retry.Timer{Timeout: time.Second * 10, Wait: time.Second}
	}
	retry.RunWith(failer(), t, func(r *retry.R) {
		ch, errCh := libservice.ServiceHealthBlockingQuery(client, serviceName, index)
		select {
		case err := <-errCh:
			require.NoError(r, err)
		case service := <-ch:
			require.Equal(r, 1, len(service))

			index = service[0].Service.ModifyIndex
			require.Equal(r, serviceName, service[0].Service.Service)
			require.Equal(r, 9998, service[0].Service.Port)
		}
	})
}
