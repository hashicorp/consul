// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package upgrade

import (
	"context"
	"fmt"
	"testing"
	"time"

	goretry "github.com/avast/retry-go"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

type testcase struct {
	oldVersion    string
	targetVersion string
	expectErr     bool
}

var (
	tcs []testcase
)

// Test upgrade a cluster of latest version to the target version
func TestStandardUpgradeToTarget_fromLatest(t *testing.T) {
	const numServers = 1
	buildOpts := &libcluster.BuildOptions{
		ConsulImageName:      utils.GetLatestImageName(),
		ConsulVersion:        utils.LatestVersion,
		Datacenter:           "dc1",
		InjectAutoEncryption: true,
	}

	cluster, _, _ := topology.NewCluster(t, &topology.ClusterConfig{
		NumServers:                numServers,
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
	err := goretry.Do(
		func() error {
			ch, errCh := libservice.ServiceHealthBlockingQuery(client, serviceName, index)
			select {
			case err := <-errCh:
				require.NoError(t, err)
			case service := <-ch:
				index = service[0].Service.ModifyIndex
				if len(service) != 1 {
					return fmt.Errorf("service is %d, want 1", len(service))
				}
				if serviceName != service[0].Service.Service {
					return fmt.Errorf("service name is %s, want %s", service[0].Service.Service, serviceName)
				}
				if service[0].Service.Port != 9998 {
					return fmt.Errorf("service is %d, want 9998", service[0].Service.Port)
				}
			}
			return nil
		},
		goretry.Attempts(5),
		goretry.Delay(time.Second),
	)
	require.NoError(t, err)

	// upgrade the cluster to the Target version
	t.Logf("initiating standard upgrade to version=%q", utils.TargetVersion)
	err = cluster.StandardUpgrade(t, context.Background(), utils.GetTargetImageName(), utils.TargetVersion)

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
