// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package session

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

// watchScript runs `consul watch` in the background with a handler that
// appends the JSON it receives to logFile, runs steps while it is watching, and
// prints what the handler saw.
func watchScript(watchArgs, logFile, steps string) string {
	return `rm -f ` + logFile + `
consul watch ` + watchArgs + ` 'cat >> ` + logFile + `; echo >> ` + logFile + `' &
WATCH=$!
sleep 4
` + steps + `
sleep 3
kill $WATCH
cat ` + logFile
}

// TestWatch_CLI covers `consul watch`, which runs a handler whenever watched
// data changes.
func TestWatch_CLI(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "", "")

	t.Run("a key watch reports each value of the key", func(t *testing.T) {
		steps := `consul kv put watch/key v1 >/dev/null
sleep 3
consul kv put watch/key v2 >/dev/null
sleep 3
consul kv delete watch/key >/dev/null`
		res := e.shell(t, watchScript("-type=key -key=watch/key", "/tmp/key-watch.log", steps))
		require.Equal(t, 0, res.ExitCode, res.Output)

		// The handler is called once with the current state (no key yet), and then
		// once per change; a missing key is reported as null.
		var seen []string
		dec := json.NewDecoder(strings.NewReader(res.Output))
		for dec.More() {
			var pair *api.KVPair
			require.NoError(t, dec.Decode(&pair), res.Output)
			if pair == nil {
				seen = append(seen, "<none>")
			} else {
				seen = append(seen, string(pair.Value))
			}
		}
		require.Equal(t, []string{"<none>", "v1", "v2", "<none>"}, seen, "the handler must see every state in order")
	})

	t.Run("a services watch reports new services", func(t *testing.T) {
		steps := `consul services register -name=watched-service -port=9000 >/dev/null`
		res := e.shell(t, watchScript("-type=services", "/tmp/services-watch.log", steps))
		require.Equal(t, 0, res.ExitCode, res.Output)

		var snapshots []map[string][]string
		dec := json.NewDecoder(strings.NewReader(res.Output))
		for dec.More() {
			var services map[string][]string
			require.NoError(t, dec.Decode(&services), res.Output)
			snapshots = append(snapshots, services)
		}
		require.GreaterOrEqual(t, len(snapshots), 2, res.Output)
		require.NotContains(t, snapshots[0], "watched-service", "the first call shows the state before the change")
		require.Contains(t, snapshots[len(snapshots)-1], "watched-service")
	})

	t.Run("an unknown watch type is rejected", func(t *testing.T) {
		res := e.shell(t, `consul watch -type=bogus cat`)

		require.NotEqual(t, 0, res.ExitCode)
		require.Contains(t, res.Output, "bogus")
	})
}

// TestExec_CLI covers `consul exec`, which runs a command on agents through the
// event system. Remote execution is disabled by default, so only the server
// agent enables it here, which also shows it is a per-agent opt-in.
func TestExec_CLI(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "disable_remote_exec = false", "")
	serverNode, err := e.server.Agent().NodeName()
	require.NoError(t, err)
	clientNode, err := e.client.Agent().NodeName()
	require.NoError(t, err)

	t.Run("a command runs on the targeted node and its output comes back", func(t *testing.T) {
		res := e.shell(t, `consul exec -wait=5s -wait-repl=1s -node=`+serverNode+` 'echo hello-from-exec'`)

		require.Equal(t, 0, res.ExitCode, res.Output)
		require.Contains(t, res.Output, serverNode+": hello-from-exec")
		require.Contains(t, res.Output, "finished with exit code 0")
		require.Contains(t, res.Output, "1 / 1 node(s) completed / acknowledged")
	})

	t.Run("only agents that enabled remote execution respond", func(t *testing.T) {
		res := e.shell(t, `consul exec -wait=5s -wait-repl=1s -node=`+clientNode+` 'echo must-not-run'`)

		require.NotContains(t, res.Output, "must-not-run")
		require.Contains(t, res.Output, "0 / 0 node(s) completed / acknowledged", "the client agent has remote execution disabled")

		res = e.shell(t, `consul exec -wait=5s -wait-repl=1s 'echo anyone-there'`)
		require.Contains(t, res.Output, serverNode+": anyone-there")
		require.NotContains(t, res.Output, clientNode+": anyone-there")
		require.Contains(t, res.Output, "1 / 1 node(s) completed / acknowledged")
	})

	t.Run("a service filter selects nodes running the service", func(t *testing.T) {
		require.NoError(t, e.server.Agent().ServiceRegister(&api.AgentServiceRegistration{Name: "exec-svc", Port: 9000}))

		res := e.shell(t, `consul exec -wait=5s -wait-repl=1s -service=exec-svc 'echo service-match'`)
		require.Contains(t, res.Output, serverNode+": service-match")

		res = e.shell(t, `consul exec -wait=5s -wait-repl=1s -service=no-such-service 'echo service-miss'`)
		require.NotContains(t, res.Output, "service-miss")
		require.Contains(t, res.Output, "0 / 0 node(s) completed / acknowledged")
	})

	t.Run("the command's exit code is reported", func(t *testing.T) {
		res := e.shell(t, `consul exec -wait=5s -wait-repl=1s -node=`+serverNode+` 'exit 4'`)

		require.Contains(t, res.Output, "finished with exit code 4")
	})

	t.Run("no job data is left behind", func(t *testing.T) {
		eventually(t, func(r *retry.R) {
			keys, _, err := e.server.KV().Keys("_rexec/", "", nil)
			require.NoError(r, err)
			require.Empty(r, keys, "the exec job's keys must be cleaned up")
		})
	})
}
