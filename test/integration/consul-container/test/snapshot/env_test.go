// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package snapshot

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
)

// env is a single server datacenter. The snapshot CLI runs inside the server's
// container, so snapshot files stay in the container.
type env struct {
	// admin is a client to the server holding the management token when ACLs
	// are enabled.
	admin *api.Client
	// mgmtToken is empty when ACLs are disabled.
	mgmtToken string

	agent libcluster.Agent
}

func newEnv(t *testing.T, acls bool) *env {
	t.Helper()

	cluster, _, _ := libtopology.NewClusterWithConfig(t, &libtopology.ClusterConfig{
		NumServers: 1,
		NumClients: 0,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter: "dc1",
			ACLEnabled: acls,
		},
	}, "", "")

	return &env{
		admin:     cluster.Agents[0].GetClient(),
		mgmtToken: cluster.TokenBootstrap,
		agent:     cluster.Agents[0],
	}
}

// cli runs `consul <args>` against the server as the holder of token, which
// may be empty, and returns the combined output and the exit code.
func (e *env) cli(t *testing.T, token string, args ...string) (string, int) {
	t.Helper()
	script := `if [ -n "$1" ]; then export CONSUL_HTTP_TOKEN="$1"; fi; shift; exec consul "$@"`
	cmd := append([]string{"sh", "-c", script, "sh", token}, args...)
	res, err := libcluster.ExecCombined(context.Background(), e.agent, cmd...)
	require.NoError(t, err)
	return res.Output, res.ExitCode
}

// tokenClient returns a client that authenticates as token.
func (e *env) tokenClient(t *testing.T, token string) *api.Client {
	t.Helper()
	c, err := e.agent.NewClient(token, false)
	require.NoError(t, err)
	return c
}

// file returns a path for a snapshot file that the CLI can write.
func (e *env) file(t *testing.T, name string) string {
	return "/tmp/" + name
}

// writeFile creates a file inside the server container.
func (e *env) writeFile(t *testing.T, path, content string) {
	t.Helper()
	res, err := libcluster.ExecCombined(context.Background(), e.agent, "sh", "-c", fmt.Sprintf("printf '%%s' '%s' > %s", content, path))
	require.NoError(t, err)
	require.Equal(t, 0, res.ExitCode, res.Output)
}
