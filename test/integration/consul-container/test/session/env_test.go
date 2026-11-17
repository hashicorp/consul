// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package session

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
)

// env is a one server + one client datacenter. Application traffic goes
// through the client agent, which forwards to the server.
type env struct {
	cluster *libcluster.Cluster
	server  *api.Client
	client  *api.Client
}

// newEnv creates the datacenter. serverHCL and clientHCL are extra agent
// configuration for the server and the client agent.
func newEnv(t *testing.T, serverHCL, clientHCL string) *env {
	t.Helper()

	cluster, _, client := libtopology.NewClusterWithConfig(t, &libtopology.ClusterConfig{
		NumServers: 1,
		NumClients: 1,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:           "dc1",
			InjectAutoEncryption: true,
		},
	}, serverHCL, clientHCL)
	return &env{cluster: cluster, server: cluster.Agents[0].GetClient(), client: client}
}

func (e *env) clientAgent() libcluster.Agent { return e.cluster.Agents[1] }

// run executes a shell script inside the client agent's container and returns
// stdout and stderr combined with the exit code. It is safe to call from any
// goroutine.
func (e *env) run(script string) (libcluster.ExecResult, error) {
	return libcluster.ExecCombined(context.Background(), e.clientAgent(), "sh", "-c", script)
}

// shell is run for the test goroutine: infrastructure errors fail the test.
func (e *env) shell(t *testing.T, script string) libcluster.ExecResult {
	t.Helper()
	res, err := e.run(script)
	require.NoError(t, err)
	return res
}

func eventually(t *testing.T, check func(r *retry.R)) {
	t.Helper()
	retry.RunWith(libcluster.LongFailer(), t, check)
}
