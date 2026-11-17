// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package security

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
)

// env is a one server + one client datacenter with gossip encryption, TLS for
// RPC and auto-encrypt, which is what the test harness enables by default.
type env struct {
	cluster *libcluster.Cluster
	server  *api.Client
	client  *api.Client
}

func newEnv(t *testing.T, serverHCL string) *env {
	t.Helper()

	cluster, _, client := libtopology.NewClusterWithConfig(t, &libtopology.ClusterConfig{
		NumServers: 1,
		NumClients: 1,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:           "dc1",
			InjectAutoEncryption: true,
		},
	}, serverHCL, "")
	return &env{cluster: cluster, server: cluster.Agents[0].GetClient(), client: client}
}

// httpsAddr returns the host:port on which the server agent's HTTPS API can be
// reached from the test process.
func (e *env) httpsAddr(t *testing.T) string {
	t.Helper()

	ctx := context.Background()
	pod := e.cluster.Agents[0].GetPod()
	host, err := pod.Host(ctx)
	require.NoError(t, err)
	port, err := pod.MappedPort(ctx, "8501/tcp")
	require.NoError(t, err)
	return net.JoinHostPort(host, port.Port())
}

// shell runs a script in the client agent's container.
func (e *env) shell(t *testing.T, script string) libcluster.ExecResult {
	t.Helper()
	res, err := libcluster.ExecCombined(context.Background(), e.cluster.Agents[1], "sh", "-c", script)
	require.NoError(t, err)
	return res
}

// caPEM is the cluster's certificate authority.
func (e *env) caPEM() string { return e.cluster.CACert }

func eventually(t *testing.T, check func(r *retry.R)) {
	t.Helper()
	retry.RunWith(libcluster.LongFailer(), t, check)
}
