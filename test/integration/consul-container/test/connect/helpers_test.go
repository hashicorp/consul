// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package connect

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
)

// env is a one server + one client datacenter. Workloads are registered on the
// client agent, and CLI commands run inside the client agent's container, which
// is where sidecars and gateways would run in a real deployment.
type env struct {
	cluster *libcluster.Cluster
	server  *api.Client
	client  *api.Client
}

func newEnv(t *testing.T) *env {
	t.Helper()

	cluster, _, client := libtopology.NewCluster(t, &libtopology.ClusterConfig{
		NumServers: 1,
		NumClients: 1,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:           "dc1",
			InjectAutoEncryption: true,
		},
	})
	return &env{cluster: cluster, server: cluster.Agents[0].GetClient(), client: client}
}

func (e *env) clientAgent() libcluster.Agent { return e.cluster.Agents[1] }

// cliStdout runs `consul <args>` on the client agent and returns stdout only.
func (e *env) cliStdout(t *testing.T, args ...string) libcluster.ExecResult {
	t.Helper()
	res, err := libcluster.ExecStdout(context.Background(), e.clientAgent(), append([]string{"consul"}, args...)...)
	require.NoError(t, err)
	return res
}

// cliCombined runs `consul <args>` on the client agent and returns stdout and
// stderr, for asserting on error messages.
func (e *env) cliCombined(t *testing.T, args ...string) libcluster.ExecResult {
	t.Helper()
	res, err := libcluster.ExecCombined(context.Background(), e.clientAgent(), append([]string{"consul"}, args...)...)
	require.NoError(t, err)
	return res
}

// shell runs a shell script on the client agent's container, for commands that
// need files or pipes. stdout and stderr are combined.
func (e *env) shell(t *testing.T, script string) libcluster.ExecResult {
	t.Helper()
	res, err := libcluster.ExecCombined(context.Background(), e.clientAgent(), "sh", "-c", script)
	require.NoError(t, err)
	return res
}

func decodeJSON(t *testing.T, s string) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(s), &out), "output was not JSON:\n%s", s)
	return out
}

// dig walks nested maps/slices: dig(m, "a", 0, "b").
func dig(t *testing.T, v any, path ...any) any {
	t.Helper()
	for _, p := range path {
		switch k := p.(type) {
		case string:
			m, ok := v.(map[string]any)
			require.True(t, ok, "expected object at %q, got %T", k, v)
			v, ok = m[k]
			require.True(t, ok, "missing key %q", k)
		case int:
			s, ok := v.([]any)
			require.True(t, ok, "expected array at index %d, got %T", k, v)
			require.Greater(t, len(s), k)
			v = s[k]
		}
	}
	return v
}

func clusterNames(t *testing.T, bootstrap map[string]any) []string {
	t.Helper()
	var names []string
	for _, c := range dig(t, bootstrap, "static_resources", "clusters").([]any) {
		names = append(names, c.(map[string]any)["name"].(string))
	}
	return names
}

func listenerNames(bootstrap map[string]any) []string {
	sr, _ := bootstrap["static_resources"].(map[string]any)
	ls, _ := sr["listeners"].([]any)
	var names []string
	for _, l := range ls {
		names = append(names, l.(map[string]any)["name"].(string))
	}
	return names
}
