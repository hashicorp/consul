package upgrade

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"

	"github.com/hashicorp/consul/sdk/testutil/retry"
	libagent "github.com/hashicorp/consul/test/integration/consul-container/libs/agent"
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
			targetVersion: *utils.TargetVersion,
		},
		{
			oldversion:    "1.14",
			targetVersion: *utils.TargetVersion,
		},
	}

	run := func(t *testing.T, tc testcase) {

		var configs []libagent.Config

		configCtx, err := libagent.NewBuildContext(libagent.BuildOptions{
			ConsulVersion: tc.oldversion,
		})
		require.NoError(t, err)
		numServers := 1
		leaderConf, err := libagent.NewConfigBuilder(configCtx).
			Bootstrap(numServers).
			ToAgentConfig()
		require.NoError(t, err)
		t.Logf("Cluster config:\n%s", leaderConf.JSON)
		leaderConf.Version = tc.oldversion
		for i := 0; i < numServers; i++ {
			configs = append(configs, *leaderConf)
		}

		cluster, err := libcluster.New(configs)
		require.NoError(t, err)
		defer terminate(t, cluster)

		server := cluster.Agents[0]
		client := server.GetClient()
		libcluster.WaitForLeader(t, cluster, client)
		libcluster.WaitForMembers(t, client, numServers)

		// Create a service to be stored in the snapshot
		serviceName := "api"
		index := serviceCreate(t, client, serviceName)
		ch := make(chan []*api.ServiceEntry)
		errCh := make(chan error)
		go func() {
			service, q, err := client.Health().Service(serviceName, "", false, &api.QueryOptions{WaitIndex: index})
			if err == nil && q.QueryBackend != api.QueryBackendStreaming {
				err = fmt.Errorf("invalid backend for this test %s", q.QueryBackend)
			}
			if err != nil {
				errCh <- err
			} else {
				ch <- service
			}
		}()
		require.NoError(t, client.Agent().ServiceRegister(
			&api.AgentServiceRegistration{Name: serviceName, Port: 9998},
		))
		timer := time.NewTimer(1 * time.Second)
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
		err = cluster.StandardUpgrade(t, context.Background(), tc.targetVersion)
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
			require.Error(t, fmt.Errorf("context deadline exceeded"))
		}
	}

	for _, tc := range tcs {
		t.Run(fmt.Sprintf("upgrade from %s to %s", tc.oldversion, tc.targetVersion),
			func(t *testing.T) {
				run(t, tc)
			})
		time.Sleep(5 * time.Second)
	}
}
