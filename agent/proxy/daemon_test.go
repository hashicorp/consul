package proxy

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/consul/testutil/retry"
	"github.com/hashicorp/go-uuid"
	"github.com/stretchr/testify/require"
)

func TestDaemon_impl(t *testing.T) {
	var _ Proxy = new(Daemon)
}

func TestDaemonStartStop(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	td, closer := testTempDir(t)
	defer closer()

	path := filepath.Join(td, "file")
	uuid, err := uuid.GenerateUUID()
	require.NoError(err)

	d := helperProcessDaemon("start-stop", path)
	d.ProxyId = "tubes"
	d.ProxyToken = uuid
	d.Logger = testLogger
	require.NoError(d.Start())
	defer d.Stop()

	// Wait for the file to exist
	retry.Run(t, func(r *retry.R) {
		_, err := os.Stat(path)
		if err == nil {
			return
		}

		r.Fatalf("error: %s", err)
	})

	// Verify that the contents of the file is the token. This verifies
	// that we properly passed the token as an env var.
	data, err := ioutil.ReadFile(path)
	require.NoError(err)
	require.Equal("tubes:"+uuid, string(data))

	// Stop the process
	require.NoError(d.Stop())

	// File should no longer exist.
	retry.Run(t, func(r *retry.R) {
		_, err := os.Stat(path)
		if os.IsNotExist(err) {
			return
		}

		// err might be nil here but that's okay
		r.Fatalf("should not exist: %s", err)
	})
}

func TestDaemonDetachesChild(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	td, closer := testTempDir(t)
	defer closer()

	path := filepath.Join(td, "file")
	pidPath := filepath.Join(td, "child.pid")

	// Start the parent process wrapping a start-stop test. The parent is acting
	// as our "agent". We need an extra indirection to be able to kill the "agent"
	// and still be running the test process.
	parentCmd := helperProcess("parent", pidPath, "start-stop", path)
	require.NoError(parentCmd.Start())

	// Wait for the pid file to exist so we know parent is running
	retry.Run(t, func(r *retry.R) {
		_, err := os.Stat(pidPath)
		if err == nil {
			return
		}

		r.Fatalf("error: %s", err)
	})

	// And wait for the actual file to be sure the child is running (it should be
	// since parent doesn't write PID until child starts but the child might not
	// have completed the write to disk yet which causes flakiness below).
	retry.Run(t, func(r *retry.R) {
		_, err := os.Stat(path)
		if err == nil {
			return
		}

		r.Fatalf("error: %s", err)
	})

	// Always cleanup child process after
	defer func() {
		_, err := os.Stat(pidPath)
		if err != nil {
			return
		}
		bs, err := ioutil.ReadFile(pidPath)
		require.NoError(err)
		pid, err := strconv.Atoi(string(bs))
		require.NoError(err)
		proc, err := os.FindProcess(pid)
		if err != nil {
			return
		}
		proc.Kill()
	}()

	time.Sleep(20 * time.Second)

	// Now kill the parent and wait for it
	require.NoError(parentCmd.Process.Kill())

	_, err := parentCmd.Process.Wait()
	require.NoError(err)

	time.Sleep(15 * time.Second)

	// The child should still be running so file should still be there AND child processid should still be there
	_, err = os.Stat(path)
	require.NoError(err, "child should still be running")

	// Let defer clean up the child process
}

func TestDaemonRestart(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	td, closer := testTempDir(t)
	defer closer()
	path := filepath.Join(td, "file")

	d := helperProcessDaemon("restart", path)
	d.Logger = testLogger
	require.NoError(d.Start())
	defer d.Stop()

	// Wait for the file to exist. We save the func so we can reuse the test.
	waitFile := func() {
		retry.Run(t, func(r *retry.R) {
			_, err := os.Stat(path)
			if err == nil {
				return
			}
			r.Fatalf("error waiting for path: %s", err)
		})
	}
	waitFile()

	// Delete the file
	require.NoError(os.Remove(path))

	// File should re-appear because the process is restart
	waitFile()
}

func TestDaemonStop_kill(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	td, closer := testTempDir(t)
	defer closer()

	path := filepath.Join(td, "file")

	d := helperProcessDaemon("stop-kill", path)
	d.ProxyToken = "hello"
	d.Logger = testLogger
	d.gracefulWait = 200 * time.Millisecond
	d.pollInterval = 100 * time.Millisecond
	require.NoError(d.Start())

	// Wait for the file to exist
	retry.Run(t, func(r *retry.R) {
		_, err := os.Stat(path)
		if err == nil {
			return
		}

		r.Fatalf("error: %s", err)
	})

	// Stop the process
	require.NoError(d.Stop())

	// Stat the file so that we can get the mtime
	fi, err := os.Stat(path)
	require.NoError(err)
	mtime := fi.ModTime()

	// The mtime shouldn't change
	time.Sleep(100 * time.Millisecond)
	fi, err = os.Stat(path)
	require.NoError(err)
	require.Equal(mtime, fi.ModTime())
}

func TestDaemonStart_pidFile(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	td, closer := testTempDir(t)

	defer closer()

	path := filepath.Join(td, "file")
	pidPath := filepath.Join(td, "pid")
	uuid, err := uuid.GenerateUUID()
	require.NoError(err)

	d := helperProcessDaemon("start-once", path)
	d.ProxyToken = uuid
	d.Logger = testLogger
	d.PidPath = pidPath
	require.NoError(d.Start())
	defer d.Stop()

	// Wait for the file to exist
	retry.Run(t, func(r *retry.R) {
		_, err := os.Stat(pidPath)
		if err == nil {
			return
		}

		r.Fatalf("error: %s", err)
	})

	// Check the pid file
	pidRaw, err := ioutil.ReadFile(pidPath)
	require.NoError(err)
	require.NotEmpty(pidRaw)

	// Stop
	require.NoError(d.Stop())

	// Pid file should be gone
	_, err = os.Stat(pidPath)
	require.True(os.IsNotExist(err))
}

// Verify the pid file changes on restart
func TestDaemonRestart_pidFile(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	td, closer := testTempDir(t)
	defer closer()
	path := filepath.Join(td, "file")
	pidPath := filepath.Join(td, "pid")

	d := helperProcessDaemon("restart", path)
	d.Logger = testLogger
	d.PidPath = pidPath
	require.NoError(d.Start())
	defer d.Stop()

	// Wait for the file to exist. We save the func so we can reuse the test.
	waitFile := func() {
		retry.Run(t, func(r *retry.R) {
			_, err := os.Stat(path)
			if err == nil {
				return
			}
			r.Fatalf("error waiting for path: %s", err)
		})
	}
	waitFile()

	// Check the pid file
	pidRaw, err := ioutil.ReadFile(pidPath)
	require.NoError(err)
	require.NotEmpty(pidRaw)

	// Delete the file
	require.NoError(os.Remove(path))

	// File should re-appear because the process is restart
	waitFile()

	// Check the pid file and it should not equal
	pidRaw2, err := ioutil.ReadFile(pidPath)
	require.NoError(err)
	require.NotEmpty(pidRaw2)
	require.NotEqual(pidRaw, pidRaw2)
}

func TestDaemonEqual(t *testing.T) {
	cases := []struct {
		Name     string
		D1, D2   Proxy
		Expected bool
	}{
		{
			"Different type",
			&Daemon{},
			&Noop{},
			false,
		},

		{
			"Nil",
			&Daemon{},
			nil,
			false,
		},

		{
			"Equal",
			&Daemon{},
			&Daemon{},
			true,
		},

		{
			"Different path",
			&Daemon{
				Path: "/foo",
			},
			&Daemon{
				Path: "/bar",
			},
			false,
		},

		{
			"Different args",
			&Daemon{
				Args: []string{"foo"},
			},
			&Daemon{
				Args: []string{"bar"},
			},
			false,
		},

		{
			"Different token",
			&Daemon{
				ProxyToken: "one",
			},
			&Daemon{
				ProxyToken: "two",
			},
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			actual := tc.D1.Equal(tc.D2)
			require.Equal(t, tc.Expected, actual)
		})
	}
}

func TestDaemonMarshalSnapshot(t *testing.T) {
	cases := []struct {
		Name     string
		Proxy    Proxy
		Expected map[string]interface{}
	}{
		{
			"stopped daemon",
			&Daemon{
				Path: "/foo",
			},
			nil,
		},

		{
			"basic",
			&Daemon{
				Path:    "/foo",
				process: &os.Process{Pid: 42},
			},
			map[string]interface{}{
				"Pid":        42,
				"Path":       "/foo",
				"Args":       []string(nil),
				"ProxyToken": "",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			actual := tc.Proxy.MarshalSnapshot()
			require.Equal(t, tc.Expected, actual)
		})
	}
}

func TestDaemonUnmarshalSnapshot(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	td, closer := testTempDir(t)
	defer closer()

	path := filepath.Join(td, "file")
	uuid, err := uuid.GenerateUUID()
	require.NoError(err)

	d := helperProcessDaemon("start-stop", path)
	d.ProxyToken = uuid
	d.Logger = testLogger
	defer d.Stop()
	require.NoError(d.Start())

	// Wait for the file to exist
	retry.Run(t, func(r *retry.R) {
		_, err := os.Stat(path)
		if err == nil {
			return
		}

		r.Fatalf("error: %s", err)
	})

	// Snapshot
	snap := d.MarshalSnapshot()

	// Stop the original daemon but keep it alive
	require.NoError(d.Close())

	// Restore the second daemon
	d2 := &Daemon{Logger: testLogger}
	require.NoError(d2.UnmarshalSnapshot(snap))

	// Stop the process
	require.NoError(d2.Stop())

	// File should no longer exist.
	retry.Run(t, func(r *retry.R) {
		_, err := os.Stat(path)
		if os.IsNotExist(err) {
			return
		}

		// err might be nil here but that's okay
		r.Fatalf("should not exist: %s", err)
	})
}

func TestDaemonUnmarshalSnapshot_notRunning(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	td, closer := testTempDir(t)
	defer closer()

	path := filepath.Join(td, "file")
	uuid, err := uuid.GenerateUUID()
	require.NoError(err)

	d := helperProcessDaemon("start-stop", path)
	d.ProxyToken = uuid
	d.Logger = testLogger
	defer d.Stop()
	require.NoError(d.Start())

	// Wait for the file to exist
	retry.Run(t, func(r *retry.R) {
		_, err := os.Stat(path)
		if err == nil {
			return
		}

		r.Fatalf("error: %s", err)
	})

	// Snapshot
	snap := d.MarshalSnapshot()

	// Stop the original daemon
	require.NoError(d.Stop())

	// Restore the second daemon
	d2 := &Daemon{Logger: testLogger}
	require.Error(d2.UnmarshalSnapshot(snap))
}
