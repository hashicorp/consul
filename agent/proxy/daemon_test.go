package proxy

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
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
		ProxyToken: uuid,
		Logger:     testLogger,
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

	// Verify that the contents of the file is the token. This verifies
	// that we properly passed the token as an env var.
	data, err := ioutil.ReadFile(path)
	require.NoError(err)
	require.Equal(uuid, string(data))

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

	// State the file so that we can get the mtime
	fi, err := os.Stat(path)
	require.NoError(err)
	mtime := fi.ModTime()

	// The mtime shouldn't change
	time.Sleep(100 * time.Millisecond)
	fi, err = os.Stat(path)
	require.NoError(err)
	require.Equal(mtime, fi.ModTime())
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
