// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package gateways

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
)

const agentGRPCAddr = "127.0.0.1:8502"

// apiEnv is a one server + one client datacenter used by the tests that
// exercise gateway configuration, catalog views, ACLs and controller status
// without running Envoy. Gateways are registered on the client agent.
type apiEnv struct {
	cluster *libcluster.Cluster
	// server is a client to the server agent, authenticated with the bootstrap
	// token when ACLs are enabled.
	server *api.Client
	// client is a client to the client agent.
	client *api.Client
}

func newAPIEnv(t *testing.T, aclEnabled bool) *apiEnv {
	t.Helper()

	cluster, _, client := libtopology.NewCluster(t, &libtopology.ClusterConfig{
		NumServers: 1,
		NumClients: 1,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter: "dc1",
			// Auto-encrypt conflicts with ACL bootstrap, so only use it without ACLs.
			InjectAutoEncryption: !aclEnabled,
			ACLEnabled:           aclEnabled,
		},
	})
	return &apiEnv{cluster: cluster, server: cluster.Agents[0].GetClient(), client: client}
}

// clientAgent is the agent that gateways register with.
func (e *apiEnv) clientAgent() libcluster.Agent { return e.cluster.Agents[1] }

// bindableAddresses returns two distinct addresses that processes on the
// client agent can bind to.
func (e *apiEnv) bindableAddresses() (string, string) {
	return "127.0.0.1", e.clientAgent().GetIP()
}

// tokenClient returns a client to the server agent that authenticates as token.
func (e *apiEnv) tokenClient(t *testing.T, token string) *api.Client {
	t.Helper()
	c, err := e.cluster.Agents[0].NewClient(token, false)
	require.NoError(t, err)
	return c
}

// cli runs `consul <args>` on the client agent and returns stdout and stderr.
func (e *apiEnv) cli(t *testing.T, args ...string) libcluster.ExecResult {
	t.Helper()
	res, err := libcluster.ExecCombined(context.Background(), e.clientAgent(), append([]string{"consul"}, args...)...)
	require.NoError(t, err)
	return res
}

func setEntry(t *testing.T, c *api.Client, entry api.ConfigEntry) {
	t.Helper()
	ok, _, err := c.ConfigEntries().Set(entry, nil)
	require.NoError(t, err)
	require.True(t, ok)
}

// requireSetFails asserts the servers refuse to store entry and that the
// error mentions wantErr.
func requireSetFails(t *testing.T, c *api.Client, entry api.ConfigEntry, wantErr string) {
	t.Helper()

	ok, _, err := c.ConfigEntries().Set(entry, nil)
	require.Error(t, err)
	require.False(t, ok)
	require.Contains(t, err.Error(), wantErr)

	_, _, err = c.ConfigEntries().Get(entry.GetKind(), entry.GetName(), nil)
	require.Error(t, err, "a rejected %s/%s must not be stored", entry.GetKind(), entry.GetName())
}
