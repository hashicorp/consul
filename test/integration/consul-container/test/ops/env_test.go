// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package ops

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

type containerNode struct {
	agent libcluster.Agent
}

func (n *containerNode) Client() *api.Client { return n.agent.GetClient() }

func (n *containerNode) HTTPAddr() string {
	cfg := n.agent.GetAPIClientConfig()
	return utils.HostPort(cfg.Address)
}

func (n *containerNode) CLI(t *testing.T, args ...string) (string, int) {
	t.Helper()
	res, err := libcluster.ExecCombined(context.Background(), n.agent, append([]string{"consul"}, args...)...)
	require.NoError(t, err)
	return res.Output, res.ExitCode
}

func (n *containerNode) WriteConfig(t *testing.T, name, content string) {
	t.Helper()
	res, err := libcluster.ExecCombined(context.Background(), n.agent,
		"sh", "-c", fmt.Sprintf("printf '%%s' '%s' > /consul/config/%s", content, name))
	require.NoError(t, err)
	require.Equal(t, 0, res.ExitCode, res.Output)
}

func (n *containerNode) RemoveConfig(t *testing.T, name string) {
	t.Helper()
	res, err := libcluster.ExecCombined(context.Background(), n.agent, "rm", "-f", "/consul/config/"+name)
	require.NoError(t, err)
	require.Equal(t, 0, res.ExitCode, res.Output)
}

// newEnv starts a datacenter of one server and one client agent.
func newEnv(t *testing.T, serverHCL, clientHCL string) *env {
	t.Helper()

	cluster, _, _ := libtopology.NewClusterWithConfig(t, &libtopology.ClusterConfig{
		NumServers: 1,
		NumClients: 1,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter: "dc1",
		},
	}, serverHCL, clientHCL)

	return &env{
		server: &containerNode{agent: cluster.Agents[0]},
		client: &containerNode{agent: cluster.Agents[1]},
	}
}
