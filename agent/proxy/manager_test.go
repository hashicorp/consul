package proxy

import (
	"io/ioutil"
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
	m, closer := testManager(t)
	defer closer()
	require.NoError(t, m.Close())

	// Close again for sanity
	require.NoError(t, m.Close())
}

// Test that Run performs an initial sync (if local.State is already set)
// rather than waiting for a notification from the local state.
func TestManagerRun_initialSync(t *testing.T) {
	t.Parallel()

	state := local.TestState(t)
	m, closer := testManager(t)
	defer closer()
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
	m, closer := testManager(t)
	defer closer()
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
	m, closer := testManager(t)
	defer closer()
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
	m, closer := testManager(t)
	defer closer()
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

func TestManagerRun_daemonLogs(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	state := local.TestState(t)
	m, closer := testManager(t)
	defer closer()
	m.State = state
	defer m.Kill()

	// Configure a log dir so that we can read the logs
	logDir := filepath.Join(m.DataDir, "logs")

	// Create the service and calculate the log paths
	path := filepath.Join(m.DataDir, "notify")
	id := testStateProxy(t, state, "web", helperProcess("output", path))
	stdoutPath := logPath(logDir, id, "stdout")
	stderrPath := logPath(logDir, id, "stderr")

	// Start the manager
	go m.Run()

	// We should see the path appear shortly
	retry.Run(t, func(r *retry.R) {
		if _, err := os.Stat(path); err != nil {
			r.Fatalf("error waiting for stdout path: %s", err)
		}
	})

	expectedOut := "hello stdout\n"
	actual, err := ioutil.ReadFile(stdoutPath)
	require.NoError(err)
	require.Equal([]byte(expectedOut), actual)

	expectedErr := "hello stderr\n"
	actual, err = ioutil.ReadFile(stderrPath)
	require.NoError(err)
	require.Equal([]byte(expectedErr), actual)
}

func TestManagerRun_daemonPid(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	state := local.TestState(t)
	m, closer := testManager(t)
	defer closer()
	m.State = state
	defer m.Kill()

	// Configure a log dir so that we can read the logs
	pidDir := filepath.Join(m.DataDir, "pids")

	// Create the service and calculate the log paths
	path := filepath.Join(m.DataDir, "notify")
	id := testStateProxy(t, state, "web", helperProcess("output", path))
	pidPath := pidPath(pidDir, id)

	// Start the manager
	go m.Run()

	// We should see the path appear shortly
	retry.Run(t, func(r *retry.R) {
		if _, err := os.Stat(path); err != nil {
			r.Fatalf("error waiting for stdout path: %s", err)
		}
	})

	// Verify the pid file is not empty
	pidRaw, err := ioutil.ReadFile(pidPath)
	require.NoError(err)
	require.NotEmpty(pidRaw)
}

// Test the Snapshot/Restore works.
func TestManagerRun_snapshotRestore(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	state := local.TestState(t)
	m, closer := testManager(t)
	defer closer()
	m.State = state
	defer m.Kill()

	// Add the proxy
	td, closer := testTempDir(t)
	defer closer()
	path := filepath.Join(td, "file")
	testStateProxy(t, state, "web", helperProcess("start-stop", path))

	// Set a low snapshot period so we get a snapshot
	m.SnapshotPeriod = 10 * time.Millisecond

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

	// Wait for the snapshot
	snapPath := m.SnapshotPath()
	retry.Run(t, func(r *retry.R) {
		raw, err := ioutil.ReadFile(snapPath)
		if err != nil {
			r.Fatalf("error waiting for path: %s", err)
		}
		if len(raw) < 30 {
			r.Fatalf("snapshot too small")
		}
	})

	// Stop the sync
	require.NoError(m.Close())

	// File should still exist
	_, err := os.Stat(path)
	require.NoError(err)

	// Restore a manager from a snapshot
	m2, closer := testManager(t)
	m2.State = state
	defer closer()
	defer m2.Kill()
	require.NoError(m2.Restore(snapPath))

	// Start
	go m2.Run()

	// Add a second proxy so that we can determine when we're up
	// and running.
	path2 := filepath.Join(td, "file")
	testStateProxy(t, state, "db", helperProcess("start-stop", path2))
	retry.Run(t, func(r *retry.R) {
		_, err := os.Stat(path2)
		if err == nil {
			return
		}
		r.Fatalf("error waiting for path: %s", err)
	})

	// Kill m2, which should kill our main process
	require.NoError(m2.Kill())

	// File should no longer exist
	retry.Run(t, func(r *retry.R) {
		_, err := os.Stat(path)
		if err != nil {
			return
		}
		r.Fatalf("file still exists")
	})
}

func testManager(t *testing.T) (*Manager, func()) {
	m := NewManager()

	// Setup a default state
	m.State = local.TestState(t)

	// Set these periods low to speed up tests
	m.CoalescePeriod = 1 * time.Millisecond
	m.QuiescentPeriod = 1 * time.Millisecond

	// Setup a temporary directory for logs
	td, closer := testTempDir(t)
	m.DataDir = td

	return m, func() { closer() }
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
	}, "token", "")
	require.NoError(t, err)

	return p.Proxy.ProxyService.ID
}
