// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package kv

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
// through the client agent, which forwards to the server, as it would in a real
// deployment.
type env struct {
	cluster *libcluster.Cluster
	// server and client are API clients to the server and client agents. With
	// ACLs enabled they carry the bootstrap token.
	server *api.Client
	client *api.Client
}

type envOpts struct {
	acl bool
	// hcl is extra agent configuration applied to both agents.
	hcl string
}

func newEnv(t *testing.T, opts envOpts) *env {
	t.Helper()

	cluster, _, client := libtopology.NewClusterWithConfig(t, &libtopology.ClusterConfig{
		NumServers: 1,
		NumClients: 1,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter: "dc1",
			// Auto-encrypt conflicts with ACL bootstrap, so only use it without ACLs.
			InjectAutoEncryption: !opts.acl,
			ACLEnabled:           opts.acl,
		},
	}, opts.hcl, opts.hcl)

	e := &env{cluster: cluster, server: cluster.Agents[0].GetClient(), client: client}
	if opts.acl {
		// The client agent's default client has no token; use the bootstrap one.
		e.client = e.tokenClient(t, cluster.TokenBootstrap)
	}
	return e
}

func (e *env) clientAgent() libcluster.Agent { return e.cluster.Agents[1] }

// tokenClient returns a client to the client agent that authenticates as token.
func (e *env) tokenClient(t *testing.T, token string) *api.Client {
	t.Helper()
	c, err := e.clientAgent().NewClient(token, false)
	require.NoError(t, err)
	return c
}

// shell runs a shell script inside the client agent's container and returns
// stdout and stderr combined with the exit code.
func (e *env) shell(t *testing.T, script string) libcluster.ExecResult {
	t.Helper()
	res, err := libcluster.ExecCombined(context.Background(), e.clientAgent(), "sh", "-c", script)
	require.NoError(t, err)
	return res
}

func eventually(t *testing.T, check func(r *retry.R)) {
	t.Helper()
	retry.RunWith(libcluster.LongFailer(), t, check)
}
