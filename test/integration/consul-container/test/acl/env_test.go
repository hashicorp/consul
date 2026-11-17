// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package acl

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
)

// env is a one server + one client datacenter with default-deny ACLs. The
// cluster is bootstrapped by the test harness, which keeps the bootstrap token.
type env struct {
	cluster *libcluster.Cluster
	// admin is a client to the server and client agents authenticated with the
	// bootstrap (global management) token.
	admin       *api.Client
	adminClient *api.Client
}

func newEnv(t *testing.T, serverHCL, clientHCL string) *env {
	t.Helper()

	cluster, _, _ := libtopology.NewClusterWithConfig(t, &libtopology.ClusterConfig{
		NumServers: 1,
		NumClients: 1,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter: "dc1",
			// Auto-encrypt conflicts with ACL bootstrap.
			InjectAutoEncryption: false,
			ACLEnabled:           true,
		},
	}, serverHCL, clientHCL)

	e := &env{cluster: cluster, admin: cluster.Agents[0].GetClient()}
	e.adminClient = e.tokenClient(t, cluster.TokenBootstrap)
	return e
}

func (e *env) serverAgent() libcluster.Agent { return e.cluster.Agents[0] }
func (e *env) clientAgent() libcluster.Agent { return e.cluster.Agents[1] }

// tokenClient returns a client to the client agent that authenticates as token.
// The client agent forwards to the server, so this is how applications reach
// Consul.
func (e *env) tokenClient(t *testing.T, token string) *api.Client {
	t.Helper()
	c, err := e.clientAgent().NewClient(token, false)
	require.NoError(t, err)
	return c
}

// newToken creates a policy with rules and a token holding only that policy,
// and returns the token and a client that authenticates with it.
func (e *env) newToken(t *testing.T, name, rules string) (*api.ACLToken, *api.Client) {
	t.Helper()

	_, _, err := e.admin.ACL().PolicyCreate(&api.ACLPolicy{Name: name, Rules: rules}, nil)
	require.NoError(t, err)
	token, _, err := e.admin.ACL().TokenCreate(&api.ACLToken{
		Description: name,
		Policies:    []*api.ACLTokenPolicyLink{{Name: name}},
	}, nil)
	require.NoError(t, err)
	return token, e.tokenClient(t, token.SecretID)
}

// writeBootstrapReset writes the reset index to the server's data directory,
// which is how Consul administrators re-enable ACL bootstrapping.
func (e *env) writeBootstrapReset(t *testing.T, index int) {
	t.Helper()
	res, err := libcluster.ExecCombined(context.Background(), e.serverAgent(),
		"sh", "-c", fmt.Sprintf("echo %d > /consul/data/acl-bootstrap-reset", index))
	require.NoError(t, err)
	require.Equal(t, 0, res.ExitCode, res.Output)
}

// shell runs a shell script inside the client agent's container, with stdout
// and stderr combined.
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

// requireDenied asserts err is an ACL denial.
func requireDenied(t retry.TestingTB, err error, msg string) {
	t.Helper()
	require.Error(t, err, msg)
	require.Contains(t, err.Error(), "403", msg)
}
