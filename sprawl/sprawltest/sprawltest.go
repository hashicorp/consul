package sprawltest

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul-topology/sprawl"
	"github.com/hashicorp/consul-topology/sprawl/internal/runner"
	"github.com/hashicorp/consul-topology/topology"
)

var skipTestTeardown bool

var cleanupPriorRunOnce sync.Once

func init() {
	if os.Getenv("SKIP_TEARDOWN") == "1" {
		skipTestTeardown = true
	}

	cleanupPriorRunOnce.Do(func() {
		fmt.Fprintf(os.Stdout, "INFO: sprawltest: triggering cleanup of any prior test runs\n")
		CleanupWorkingDirectories()
	})
}

// Launch will create the topology defined by the provided configuration and
// bring up all of the relevant clusters.
//
// - Logs will be routed to (*testing.T).Logf.
//
//   - By default everything will be stopped and removed via
//     (*testing.T).Cleanup. For failed tests, this can be skipped by setting the
//     environment variable SKIP_TEARDOWN=1.
func Launch(t *testing.T, cfg *topology.Config) *sprawl.Sprawl {
	SkipIfTerraformNotPresent(t)
	sp, err := sprawl.Launch(
		testutil.Logger(t),
		initWorkingDirectory(t),
		cfg,
	)
	require.NoError(t, err)
	stopOnCleanup(t, sp)
	return sp
}

func initWorkingDirectory(t *testing.T) string {
	scratchDir := filepath.Join("workdir", t.Name())
	_ = os.RemoveAll(scratchDir) // cleanup prior runs
	if err := os.MkdirAll(scratchDir, 0755); err != nil {
		t.Fatalf("error: %v", err)
	}

	t.Cleanup(func() {
		if t.Failed() && !skipTestTeardown {
			t.Logf("test failed; leaving sprawl terraform definitions in: %s", scratchDir)
		} else {
			_ = os.RemoveAll(scratchDir)
		}
	})

	return scratchDir
}

func stopOnCleanup(t *testing.T, sp *sprawl.Sprawl) {
	t.Cleanup(func() {
		if t.Failed() && !skipTestTeardown {
			t.Log("test failed; leaving sprawl running")
		} else {
			//nolint:errcheck
			sp.Stop()
		}
	})
}

// CleanupWorkingDirectories is meant to run in an init() once at the start of
// any tests.
func CleanupWorkingDirectories() {
	fi, err := os.ReadDir("workdir")
	if os.IsNotExist(err) {
		return
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "WARN: sprawltest: unable to scan 'workdir' for prior runs to cleanup\n")
		return
	} else if len(fi) == 0 {
		fmt.Fprintf(os.Stdout, "INFO: sprawltest: no prior tests to clean up\n")
		return
	}

	r, err := runner.Load(hclog.NewNullLogger())
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARN: sprawltest: unable to look for 'terraform' and 'docker' binaries\n")
		return
	}

	for _, d := range fi {
		if !d.IsDir() {
			continue
		}
		path := filepath.Join("workdir", d.Name(), "terraform")

		fmt.Fprintf(os.Stdout, "INFO: sprawltest: cleaning up failed prior run in: %s\n", path)

		err := r.TerraformExec([]string{
			"init", "-input=false",
		}, io.Discard, path)

		err2 := r.TerraformExec([]string{
			"destroy", "-input=false", "-auto-approve", "-refresh=false",
		}, io.Discard, path)

		if err2 != nil {
			err = multierror.Append(err, err2)
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "WARN: sprawltest: could not clean up failed prior run in: %s: %v\n", path, err)
		} else {
			_ = os.RemoveAll(path)
		}
	}
}

func SkipIfTerraformNotPresent(t *testing.T) {
	const terraformBinaryName = "terraform"

	path, err := exec.LookPath(terraformBinaryName)
	if err != nil || path == "" {
		t.Skipf("%q not found on $PATH - download and install to run this test", terraformBinaryName)
	}
}
