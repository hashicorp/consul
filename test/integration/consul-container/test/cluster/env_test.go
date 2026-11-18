// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package cluster

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
)

type containerServer struct {
	agent libcluster.Agent
}

func (s *containerServer) Name() string        { return s.agent.GetAgentName() }
func (s *containerServer) Client() *api.Client { return s.agent.GetClient() }

func (s *containerServer) Halt(t *testing.T) {
	t.Helper()
	require.NoError(t, s.agent.TerminateAndRetainPod(true))
}

func (s *containerServer) Start(t *testing.T) {
	t.Helper()
	require.NoError(t, s.agent.Upgrade(context.Background(), s.agent.GetConfig()))
}

func (s *containerServer) CLI(t *testing.T, args ...string) (string, int) {
	t.Helper()
	res, err := libcluster.ExecCombined(context.Background(), s.agent, append([]string{"consul"}, args...)...)
	require.NoError(t, err)
	return res.Output, res.ExitCode
}

func (s *containerServer) WriteDataFile(t *testing.T, name, content string) {
	t.Helper()
	res, err := libcluster.ExecCombined(context.Background(), s.agent,
		"sh", "-c", fmt.Sprintf("printf '%%s' '%s' > /consul/data/%s", content, name))
	require.NoError(t, err)
	require.Equal(t, 0, res.ExitCode, res.Output)
}

// newTestCluster starts a datacenter with numServers servers. serverHCL is
// applied to every server, including servers added later.
func newTestCluster(t *testing.T, numServers int, serverHCL string) *testCluster {
	t.Helper()

	cluster, buildCtx, _ := libtopology.NewClusterWithConfig(t, &libtopology.ClusterConfig{
		NumServers: numServers,
		NumClients: 0,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter: "dc1",
		},
	}, serverHCL, "")

	tc := &testCluster{}
	for _, agent := range cluster.Agents {
		tc.servers = append(tc.servers, &containerServer{agent: agent})
	}

	tc.add = func(t *testing.T) server {
		t.Helper()
		conf := libcluster.NewConfigBuilder(buildCtx).ToAgentConfig(t)
		if serverHCL != "" {
			conf.MutatebyAgentConfig(serverHCL)
		}
		require.NoError(t, cluster.Add([]libcluster.Config{*conf}, true))
		agents := cluster.Agents
		return &containerServer{agent: agents[len(agents)-1]}
	}
	return tc
}
