package upgrade

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

// Test upgrade a cluster of latest version to the target version
func TestStandardUpgradeToTarget_fromLatest(t *testing.T) {

	type testcase struct {
		oldversion    string
		targetVersion string
		expectErr     bool
	}
	tcs := []testcase{
		// Use the case of "1.12.3" ==> "1.13.0" to verify the test can
		// catch the upgrade bug found in snapshot of 1.13.0
		{
			oldversion:    "1.12.3",
			targetVersion: "1.13.0",
			expectErr:     true,
		},
		{
			oldversion:    "1.13",
			targetVersion: utils.TargetVersion,
		},
		{
			oldversion:    "1.14",
			targetVersion: utils.TargetVersion,
		},
	}

	run := func(t *testing.T, tc testcase) {
		configCtx := libcluster.NewBuildContext(t, libcluster.BuildOptions{
			ConsulImageName: utils.GetLatestImageName(),
			ConsulVersion:   tc.oldversion,
		})

		const (
			numServers = 1
		)

		serverConf := libcluster.NewConfigBuilder(configCtx).
			Bootstrap(numServers).
			ToAgentConfig(t)
		t.Logf("Cluster config:\n%s", serverConf.JSON)
		require.Equal(t, tc.oldversion, serverConf.Version) // TODO: remove

		cluster, err := libcluster.NewN(t, *serverConf, numServers)
		require.NoError(t, err)

		client := cluster.APIClient(0)

		libcluster.WaitForLeader(t, cluster, client)
		libcluster.WaitForMembers(t, client, numServers)

		// Create a service to be stored in the snapshot
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

		// upgrade the cluster to the Target version
		t.Logf("initiating standard upgrade to version=%q", tc.targetVersion)
		err = cluster.StandardUpgrade(t, context.Background(), utils.GetTargetImageName(), tc.targetVersion)
		if !tc.expectErr {
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
		} else {
			require.ErrorContains(t, err, "context deadline exceeded")
		}
	}

	for _, tc := range tcs {
		t.Run(fmt.Sprintf("upgrade from %s to %s", tc.oldversion, tc.targetVersion),
			func(t *testing.T) {
				run(t, tc)
			})
		// time.Sleep(5 * time.Second)
	}
}
