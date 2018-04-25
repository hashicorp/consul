package proxy

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/consul/testutil/retry"
	"github.com/hashicorp/go-uuid"
	"github.com/stretchr/testify/require"
)

func TestDaemonStartStop(t *testing.T) {
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
