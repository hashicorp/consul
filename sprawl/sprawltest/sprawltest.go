package sprawltest

import (
	"context"
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

// TODO(rb): move comments to doc.go

var (
	// set SPRAWL_WORKDIR_ROOT in the environment to have the test output
	// coalesced in here. By default it uses a directory called "workdir" in
	// each package.
	workdirRoot string

	// set SPRAWL_KEEP_WORKDIR=1 in the environment to keep the workdir output
	// intact. Files are all destroyed by default.
	keepWorkdirOnFail bool

	// set SPRAWL_KEEP_RUNNING=1 in the environment to keep the workdir output
	// intact and also refrain from tearing anything down. Things are all
	// destroyed by default.
	//
	// SPRAWL_KEEP_RUNNING=1 implies SPRAWL_KEEP_WORKDIR=1
	keepRunningOnFail bool

	// set SPRAWL_SKIP_OLD_CLEANUP to prevent the library from tearing down and
	// removing anything found in the working directory at init time. The
	// default behavior is to do this.
	skipOldCleanup bool
)

var cleanupPriorRunOnce sync.Once

func init() {
	if root := os.Getenv("SPRAWL_WORKDIR_ROOT"); root != "" {
		fmt.Fprintf(os.Stdout, "INFO: sprawltest: SPRAWL_WORKDIR_ROOT set; using %q as output root\n", root)
		workdirRoot = root
	} else {
		workdirRoot = "workdir"
	}

	if os.Getenv("SPRAWL_KEEP_WORKDIR") == "1" {
		keepWorkdirOnFail = true
		fmt.Fprintf(os.Stdout, "INFO: sprawltest: SPRAWL_KEEP_WORKDIR set; not destroying workdir on failure\n")
	}

	if os.Getenv("SPRAWL_KEEP_RUNNING") == "1" {
		keepRunningOnFail = true
		keepWorkdirOnFail = true
		fmt.Fprintf(os.Stdout, "INFO: sprawltest: SPRAWL_KEEP_RUNNING set; not tearing down resources on failure\n")
	}

	if os.Getenv("SPRAWL_SKIP_OLD_CLEANUP") == "1" {
		skipOldCleanup = true
		fmt.Fprintf(os.Stdout, "INFO: sprawltest: SPRAWL_SKIP_OLD_CLEANUP set; not cleaning up anything found in %q\n", workdirRoot)
	}

	if !skipOldCleanup {
		cleanupPriorRunOnce.Do(func() {
			fmt.Fprintf(os.Stdout, "INFO: sprawltest: triggering cleanup of any prior test runs\n")
			CleanupWorkingDirectories()
		})
	}
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
	// TODO(rb): figure out how to get the calling package which we can put in
	// the middle here, which is likely 2 call frames away so maybe
	// runtime.Callers can help
	scratchDir := filepath.Join(workdirRoot, t.Name())
	_ = os.RemoveAll(scratchDir) // cleanup prior runs
	if err := os.MkdirAll(scratchDir, 0755); err != nil {
		t.Fatalf("error: %v", err)
	}

	t.Cleanup(func() {
		if t.Failed() && keepWorkdirOnFail {
			t.Logf("test failed; leaving sprawl terraform definitions in: %s", scratchDir)
		} else {
			_ = os.RemoveAll(scratchDir)
		}
	})

	return scratchDir
}

func stopOnCleanup(t *testing.T, sp *sprawl.Sprawl) {
	t.Cleanup(func() {
		if t.Failed() && keepWorkdirOnFail {
			// It's only worth it to capture the logs if we aren't going to
			// immediately discard them.
			if err := sp.CaptureLogs(context.Background()); err != nil {
				t.Logf("log capture encountered failures: %v", err)
			}
			if err := sp.SnapshotEnvoy(context.Background()); err != nil {
				t.Logf("envoy snapshot capture encountered failures: %v", err)
			}
		}

		if t.Failed() && keepRunningOnFail {
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
	fi, err := os.ReadDir(workdirRoot)
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

	ctx := context.Background()

	for _, d := range fi {
		if !d.IsDir() {
			continue
		}
		path := filepath.Join(workdirRoot, d.Name(), "terraform")

		fmt.Fprintf(os.Stdout, "INFO: sprawltest: cleaning up failed prior run in: %s\n", path)

		err := r.TerraformExec(ctx, []string{
			"init", "-input=false",
		}, io.Discard, path)

		err2 := r.TerraformExec(ctx, []string{
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
