package proxy

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/consul/testutil/retry"
	"github.com/stretchr/testify/require"
)

func TestExternalWait(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	td, closer := testTempDir(t)
	defer closer()
	path := filepath.Join(td, "file")

	cmd := helperProcess("restart", path)
	require.NoError(cmd.Start())
	exitCh := make(chan struct{})
	// Launch waiter to make sure this process isn't zombified when it exits part
	// way through the test.
	go func() {
		cmd.Process.Wait()
		close(exitCh)
	}()
	defer cmd.Process.Kill()

	// Create waiter
	pollInterval := 1 * time.Millisecond
	waitCh, closer := externalWait(cmd.Process.Pid, pollInterval)
	defer closer()

	// Wait for the file to exist so we don't rely on timing to not race with
	// process startup.
	retry.Run(t, func(r *retry.R) {
		_, err := os.Stat(path)
		if err == nil {
			return
		}

		r.Fatalf("error: %s", err)
	})

	// waitCh should not be closed until process quits. We'll wait a bit to verify
	// we weren't just too quick to see a process exit
	select {
	case <-waitCh:
		t.Fatal("waitCh should not be closed yet")
	default:
	}

	// Delete the file
	require.NoError(os.Remove(path))

	// Wait for the child to actually exit cleanly
	<-exitCh

	// Now we _should_ see waitCh close (need to wait at least a whole poll
	// interval)
	select {
	case <-waitCh:
		// OK
	case <-time.After(10 * pollInterval):
		t.Fatal("waitCh should be closed")
	}
}
