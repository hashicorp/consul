package proxy

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hashicorp/consul/agent/local"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/stretchr/testify/require"
)

func TestManagerClose_noRun(t *testing.T) {
	t.Parallel()

	// Really we're testing that it doesn't deadlock here.
	m := NewManager()
	require.NoError(t, m.Close())

	// Close again for sanity
	require.NoError(t, m.Close())
}

// Test that Run performs an initial sync (if local.State is already set)
// rather than waiting for a notification from the local state.
func TestManagerRun_initialSync(t *testing.T) {
	t.Parallel()

	state := testState(t)
	m := NewManager()
	m.State = state
	defer m.Kill()

	// Add the proxy before we start the manager to verify initial sync
	td, closer := testTempDir(t)
	defer closer()
	path := filepath.Join(td, "file")
	testStateProxy(t, state, helperProcess("restart", path))

	// Start the manager
	go m.Run()

	// We should see the path appear shortly
	retry.Run(t, func(r *retry.R) {
		_, err := os.Stat(path)
		if err == nil {
			return
		}
		r.Fatalf("error waiting for path: %s", err)
	})
}

func testState(t *testing.T) *local.State {
	state := local.TestState(t)
	require.NoError(t, state.AddService(&structs.NodeService{
		Service: "web",
	}, "web"))

	return state
}

// testStateProxy registers a proxy with the given local state and the command
// (expected to be from the helperProcess function call). It returns the
// ID for deregistration.
func testStateProxy(t *testing.T, state *local.State, cmd *exec.Cmd) string {
	command := []string{cmd.Path}
	command = append(command, cmd.Args...)

	p, err := state.AddProxy(&structs.ConnectManagedProxy{
		ExecMode:        structs.ProxyExecModeDaemon,
		Command:         command,
		TargetServiceID: "web",
	}, "web")
	require.NoError(t, err)

	return p.Proxy.ProxyService.ID
}
