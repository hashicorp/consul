package proxy

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

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

	state := local.TestState(t)
	m := NewManager()
	m.State = state
	defer m.Kill()

	// Add the proxy before we start the manager to verify initial sync
	td, closer := testTempDir(t)
	defer closer()
	path := filepath.Join(td, "file")
	testStateProxy(t, state, "web", helperProcess("restart", path))

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

func TestManagerRun_syncNew(t *testing.T) {
	t.Parallel()

	state := local.TestState(t)
	m := NewManager()
	m.State = state
	defer m.Kill()

	// Start the manager
	go m.Run()

	// Sleep a bit, this is just an attempt for Run to already be running.
	// Its not a big deal if this sleep doesn't happen (slow CI).
	time.Sleep(100 * time.Millisecond)

	// Add the first proxy
	td, closer := testTempDir(t)
	defer closer()
	path := filepath.Join(td, "file")
	testStateProxy(t, state, "web", helperProcess("restart", path))

	// We should see the path appear shortly
	retry.Run(t, func(r *retry.R) {
		_, err := os.Stat(path)
		if err == nil {
			return
		}
		r.Fatalf("error waiting for path: %s", err)
	})

	// Add another proxy
	path = path + "2"
	testStateProxy(t, state, "db", helperProcess("restart", path))
	retry.Run(t, func(r *retry.R) {
		_, err := os.Stat(path)
		if err == nil {
			return
		}
		r.Fatalf("error waiting for path: %s", err)
	})
}

func TestManagerRun_syncDelete(t *testing.T) {
	t.Parallel()

	state := local.TestState(t)
	m := NewManager()
	m.State = state
	defer m.Kill()

	// Start the manager
	go m.Run()

	// Add the first proxy
	td, closer := testTempDir(t)
	defer closer()
	path := filepath.Join(td, "file")
	id := testStateProxy(t, state, "web", helperProcess("restart", path))

	// We should see the path appear shortly
	retry.Run(t, func(r *retry.R) {
		_, err := os.Stat(path)
		if err == nil {
			return
		}
		r.Fatalf("error waiting for path: %s", err)
	})

	// Remove the proxy
	_, err := state.RemoveProxy(id)
	require.NoError(t, err)

	// File should disappear as process is killed
	retry.Run(t, func(r *retry.R) {
		_, err := os.Stat(path)
		if err == nil {
			r.Fatalf("path exists")
		}
	})
}

func TestManagerRun_syncUpdate(t *testing.T) {
	t.Parallel()

	state := local.TestState(t)
	m := NewManager()
	m.State = state
	defer m.Kill()

	// Start the manager
	go m.Run()

	// Add the first proxy
	td, closer := testTempDir(t)
	defer closer()
	path := filepath.Join(td, "file")
	testStateProxy(t, state, "web", helperProcess("restart", path))

	// We should see the path appear shortly
	retry.Run(t, func(r *retry.R) {
		_, err := os.Stat(path)
		if err == nil {
			return
		}
		r.Fatalf("error waiting for path: %s", err)
	})

	// Update the proxy with a new path
	oldPath := path
	path = path + "2"
	testStateProxy(t, state, "web", helperProcess("restart", path))
	retry.Run(t, func(r *retry.R) {
		_, err := os.Stat(path)
		if err == nil {
			return
		}
		r.Fatalf("error waiting for path: %s", err)
	})

	// Old path should be gone
	retry.Run(t, func(r *retry.R) {
		_, err := os.Stat(oldPath)
		if err == nil {
			r.Fatalf("old path exists")
		}
	})
}

// testStateProxy registers a proxy with the given local state and the command
// (expected to be from the helperProcess function call). It returns the
// ID for deregistration.
func testStateProxy(t *testing.T, state *local.State, service string, cmd *exec.Cmd) string {
	command := []string{cmd.Path}
	command = append(command, cmd.Args...)

	require.NoError(t, state.AddService(&structs.NodeService{
		Service: service,
	}, "token"))

	p, err := state.AddProxy(&structs.ConnectManagedProxy{
		ExecMode:        structs.ProxyExecModeDaemon,
		Command:         command,
		TargetServiceID: service,
	}, "token")
	require.NoError(t, err)

	return p.Proxy.ProxyService.ID
}
