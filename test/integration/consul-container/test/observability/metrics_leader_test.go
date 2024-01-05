// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package observability

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
)

// Given a 3-server cluster, when the leader is elected, then leader's isLeader is 1 and non-leader's 0
func TestLeadershipMetrics(t *testing.T) {
	t.Parallel()

	opts := libcluster.BuildOptions{
		InjectAutoEncryption:   true,
		InjectGossipEncryption: true,
	}
	ctx := libcluster.NewBuildContext(t, opts)

	var configs []libcluster.Config

	statsConf := libcluster.NewConfigBuilder(ctx).
		Telemetry("127.0.0.0:2180").
		ToAgentConfig(t)

	configs = append(configs, *statsConf)

	conf := libcluster.NewConfigBuilder(ctx).
		Bootstrap(3).
		ToAgentConfig(t)

	numServer := 3
	for i := 1; i < numServer; i++ {
		configs = append(configs, *conf)
	}

	cluster, err := libcluster.New(t, configs)
	require.NoError(t, err)

	svrCli := cluster.Agents[0].GetClient()
	libcluster.WaitForLeader(t, cluster, svrCli)
	libcluster.WaitForMembers(t, svrCli, 3)

	leader, err := cluster.Leader()
	require.NoError(t, err)
	leadAddr := leader.GetIP()

	for _, agent := range cluster.Agents {
		client := agent.GetClient().Agent()

		retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
			info, err := client.Metrics()
			require.NoError(r, err)

			var (
				leaderGauge api.GaugeValue
				found       bool
			)
			for _, g := range info.Gauges {
				if strings.HasSuffix(g.Name, ".server.isLeader") {
					leaderGauge = g
					found = true
				}
			}
			require.True(r, found, "did not find isLeader gauge metric")

			addr := agent.GetIP()
			if addr == leadAddr {
				require.Equal(r, float32(1), leaderGauge.Value)
			} else {
				require.Equal(r, float32(0), leaderGauge.Value)
			}
		})
	}
}
