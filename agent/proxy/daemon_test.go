package proxy

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
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

	d := &Daemon{
		Command:    helperProcess("start-stop", path),
		ProxyID:    "tubes",
		ProxyToken: uuid,
		Logger:     testLogger,
	}
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

func TestDaemonRestart(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	td, closer := testTempDir(t)
	defer closer()
	path := filepath.Join(td, "file")

	d := &Daemon{
		Command: helperProcess("restart", path),
		Logger:  testLogger,
	}
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

func TestDaemonLaunchesNewProcessGroup(t *testing.T) {
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

	// We MUST run this as a separate process group otherwise the Kill below will
	// kill this test process (and possibly your shell/editor that launched it!)
	parentCmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

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

	// Get the child PID
	bs, err := ioutil.ReadFile(pidPath)
	require.NoError(err)
	pid, err := strconv.Atoi(string(bs))
	require.NoError(err)
	proc, err := os.FindProcess(pid)
	require.NoError(err)

	// Always cleanup child process after
	defer func() {
		if proc != nil {
			proc.Kill()
		}
	}()

	// Now kill the parent's whole process group and wait for it
	pgid, err := syscall.Getpgid(parentCmd.Process.Pid)

	require.NoError(err)
	// Yep the minus PGid is how you kill a whole process group in unix... no idea
	// how this works on windows. We TERM no KILL since we rely on the child
	// catching the signal and deleting it's file to detect correct behaviour.
	require.NoError(syscall.Kill(-pgid, syscall.SIGTERM))

	_, err = parentCmd.Process.Wait()
	require.NoError(err)

	// The child should still be running so file should still be there
	_, err = os.Stat(path)
	require.NoError(err, "child should still be running")

	// TEST PART 2 - verify that adopting an existing process works and picks up
	// monitoring even though it's not a child. We can't do this accurately with
	// Restart test since even if we create a new `Daemon` object the test process
	// is still the parent. We need the indirection of the `parent` test helper to
	// actually verify "adoption" on restart works.

	// Start a new parent that will "adopt" the existing child even though it will
	// not be an actual child process.
	fosterCmd := helperProcess("parent", pidPath, "start-stop", path)
	// Don't care about it being same process group this time as we will just kill
	// it normally.
	require.NoError(fosterCmd.Start())
	defer func() {
		// Clean up the daemon and wait for it to prevent it becoming a zombie.
		fosterCmd.Process.Kill()
		fosterCmd.Wait()
	}()

	// The child should still be running so file should still be there
	_, err = os.Stat(path)
	require.NoError(err, "child should still be running")

	{
		// Get the child PID - it shouldn't have changed and should be running
		bs2, err := ioutil.ReadFile(pidPath)
		require.NoError(err)
		pid2, err := strconv.Atoi(string(bs2))
		require.NoError(err)
		// Defer a cleanup (til end of test function)
		proc, err := os.FindProcess(pid)
		require.NoError(err)
		defer func() { proc.Kill() }()

		require.Equal(pid, pid2)
		t.Logf("Child PID was %d and still %d", pid, pid2)
	}

	// Now killing the child directly should still be restarted by the Daemon
	require.NoError(proc.Kill())
	proc = nil

	retry.Run(t, func(r *retry.R) {
		// Get the child PID - it should have changed
		bs, err := ioutil.ReadFile(pidPath)
		r.Check(err)

		newPid, err := strconv.Atoi(string(bs))
		r.Check(err)
		if newPid == pid {
			r.Fatalf("Child PID file not changed, Daemon not restarting it")
		}
		t.Logf("Child PID was %d and is now %d", pid, newPid)
	})

	// I had to run through this test in debugger a lot of times checking ps state
	// by hand at different points to convince myself it was doing the right
	// thing. It doesn't help that with verbose logs on it seems that the stdio
	// from the `parent` process can sometimes miss lines out due to timing. For
	// example the `[INFO] agent/proxy: daemon exited...` log from Daemon that
	// indicates that the child was detected to have failed and is restarting is
	// never output on my Mac at full speed. But if I run in debugger and have it
	// pause at the step after the child is killed above, then it shows. The
	// `[DEBUG] agent/proxy: starting proxy:` for the restart does always come
	// through though which is odd. I assume this is some odd quirk of timing
	// between processes and stdio or something but it makes debugging this stuff
	// even harder!

	// Let defer clean up the child process(es)
}

func TestDaemonStop_kill(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	td, closer := testTempDir(t)
	defer closer()

	path := filepath.Join(td, "file")

	d := &Daemon{
		Command:      helperProcess("stop-kill", path),
		ProxyToken:   "hello",
		Logger:       testLogger,
		gracefulWait: 200 * time.Millisecond,
	}
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

func TestDaemonStop_killAdopted(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	td, closer := testTempDir(t)
	defer closer()

	path := filepath.Join(td, "file")

	// In this test we want to ensure that gracefull/ungraceful stop works with
	// processes that were adopted by current process but not started by it. (i.e.
	// we have to poll them not use Wait).
	//
	// We could use `parent` indirection to get a child that is actually not
	// started by this process but that's a lot of hoops to jump through on top of
	// an already complex multi-process test case.
	//
	// For now we rely on an implementation detail of Daemon which is potentially
	// brittle but beats lots of extra complexity here. Currently, if
	// Daemon.process is non-nil, the keepAlive loop will explicitly assume it's
	// not a child and so will use polling to monitor it. If we ever change that
	// it might invalidate this test and we would either need more indirection
	// here, or an alternative explicit signal on Daemon like Daemon.forcePoll to
	// ensure we are exercising that code path.

	// Start the "child" process
	childCmd := helperProcess("stop-kill", path)
	require.NoError(childCmd.Start())
	go func() { childCmd.Wait() }() // Prevent it becoming a zombie when killed
	defer func() { childCmd.Process.Kill() }()

	// Create the Daemon
	d := &Daemon{
		Command:      helperProcess("stop-kill", path),
		ProxyToken:   "hello",
		Logger:       testLogger,
		gracefulWait: 200 * time.Millisecond,
		// Can't just set process as it will bypass intializing stopCh etc.
	}
	// Adopt the pid from a fake state snapshot (this correctly initialises Daemon
	// for adoption)
	fakeSnap := map[string]interface{}{
		"Pid":         childCmd.Process.Pid,
		"CommandPath": childCmd.Path,
		"CommandArgs": childCmd.Args,
		"CommandDir":  childCmd.Dir,
		"CommandEnv":  childCmd.Env,
		"ProxyToken":  d.ProxyToken,
	}
	require.NoError(d.UnmarshalSnapshot(fakeSnap))
	require.NoError(d.Start())

	// Wait for the file to exist (child was already running so this doesn't
	// gaurantee that Daemon is in "polling" state)
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

	d := &Daemon{
		Command:    helperProcess("start-once", path),
		ProxyToken: uuid,
		Logger:     testLogger,
		PidPath:    pidPath,
	}
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

	d := &Daemon{
		Command: helperProcess("restart", path),
		Logger:  testLogger,
		PidPath: pidPath,
	}
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
			&Daemon{
				Command: &exec.Cmd{},
			},
			&Noop{},
			false,
		},

		{
			"Nil",
			&Daemon{
				Command: &exec.Cmd{},
			},
			nil,
			false,
		},

		{
			"Equal",
			&Daemon{
				Command: &exec.Cmd{},
			},
			&Daemon{
				Command: &exec.Cmd{},
			},
			true,
		},

		{
			"Different proxy ID",
			&Daemon{
				Command: &exec.Cmd{Path: "/foo"},
				ProxyID: "web",
			},
			&Daemon{
				Command: &exec.Cmd{Path: "/foo"},
				ProxyID: "db",
			},
			false,
		},

		{
			"Different path",
			&Daemon{
				Command: &exec.Cmd{Path: "/foo"},
			},
			&Daemon{
				Command: &exec.Cmd{Path: "/bar"},
			},
			false,
		},

		{
			"Different dir",
			&Daemon{
				Command: &exec.Cmd{Dir: "/foo"},
			},
			&Daemon{
				Command: &exec.Cmd{Dir: "/bar"},
			},
			false,
		},

		{
			"Different args",
			&Daemon{
				Command: &exec.Cmd{Args: []string{"foo"}},
			},
			&Daemon{
				Command: &exec.Cmd{Args: []string{"bar"}},
			},
			false,
		},

		{
			"Different token",
			&Daemon{
				Command:    &exec.Cmd{},
				ProxyToken: "one",
			},
			&Daemon{
				Command:    &exec.Cmd{},
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
				Command: &exec.Cmd{Path: "/foo"},
			},
			nil,
		},

		{
			"basic",
			&Daemon{
				Command: &exec.Cmd{Path: "/foo"},
				ProxyID: "web",
				process: &os.Process{Pid: 42},
			},
			map[string]interface{}{
				"Pid":         42,
				"CommandPath": "/foo",
				"CommandArgs": []string(nil),
				"CommandDir":  "",
				"CommandEnv":  []string(nil),
				"ProxyToken":  "",
				"ProxyID":     "web",
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

	d := &Daemon{
		Command:    helperProcess("start-stop", path),
		ProxyToken: uuid,
		Logger:     testLogger,
	}
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

	// Verify the daemon is still running
	_, err = os.Stat(path)
	require.NoError(err)

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

	d := &Daemon{
		Command:    helperProcess("start-stop", path),
		ProxyToken: uuid,
		Logger:     testLogger,
	}
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
