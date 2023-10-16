// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"testing"
	"time"

	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/stretchr/testify/require"
)

func TestStringHashSHA256(t *testing.T) {
	t.Parallel()
	in := "hello world\n"
	expected := "a948904f2f0f479b8f8197694b30184b0d2ed1c1cd2a1ec0fb85d299a192a447"

	if out := stringHashSHA256(in); out != expected {
		t.Fatalf("bad: %s expected %s", out, expected)
	}
}

func TestSetFilePermissions(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.SkipNow()
	}
	tempFile := testutil.TempFile(t, "consul")
	path := tempFile.Name()

	// Bad UID fails
	if err := setFilePermissions(path, "%", "", ""); err == nil {
		t.Fatalf("should fail")
	}

	// Bad GID fails
	if err := setFilePermissions(path, "", "%", ""); err == nil {
		t.Fatalf("should fail")
	}

	// Bad mode fails
	if err := setFilePermissions(path, "", "", "%"); err == nil {
		t.Fatalf("should fail")
	}

	// Allows omitting user/group/mode
	if err := setFilePermissions(path, "", "", ""); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Doesn't change mode if not given
	if err := os.Chmod(path, 0700); err != nil {
		t.Fatalf("err: %s", err)
	}
	if err := setFilePermissions(path, "", "", ""); err != nil {
		t.Fatalf("err: %s", err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if fi.Mode().String() != "-rwx------" {
		t.Fatalf("bad: %s", fi.Mode())
	}

	// Changes mode if given
	if err := setFilePermissions(path, "", "", "0777"); err != nil {
		t.Fatalf("err: %s", err)
	}
	fi, err = os.Stat(path)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if fi.Mode().String() != "-rwxrwxrwx" {
		t.Fatalf("bad: %s", fi.Mode())
	}
}

func TestDurationFixer(t *testing.T) {
	obj := map[string]interface{}{
		"key1": []map[string]interface{}{
			{
				"subkey1": "10s",
			},
			{
				"subkey2": "5d",
			},
		},
		"key2": map[string]interface{}{
			"subkey3": "30s",
			"subkey4": "20m",
		},
		"key3": "11s",
		"key4": "49h",
	}
	expected := map[string]interface{}{
		"key1": []map[string]interface{}{
			{
				"subkey1": 10 * time.Second,
			},
			{
				"subkey2": "5d",
			},
		},
		"key2": map[string]interface{}{
			"subkey3": "30s",
			"subkey4": 20 * time.Minute,
		},
		"key3": "11s",
		"key4": 49 * time.Hour,
	}

	fixer := NewDurationFixer("key4", "subkey1", "subkey4")
	if err := fixer.FixupDurations(obj); err != nil {
		t.Fatal(err)
	}

	// Ensure we only processed the intended fieldnames
	require.Equal(t, expected, obj)
}

// helperProcessSentinel is a sentinel value that is put as the first
// argument following "--" and is used to determine if TestHelperProcess
// should run.
const helperProcessSentinel = "GO_WANT_HELPER_PROCESS"

// helperProcess returns an *exec.Cmd that can be used to execute the
// TestHelperProcess function below. This can be used to test multi-process
// interactions.
func helperProcess(s ...string) (*exec.Cmd, func()) {
	cs := []string{"-test.run=TestHelperProcess", "--", helperProcessSentinel}
	cs = append(cs, s...)

	cmd := exec.Command(os.Args[0], cs...)
	destroy := func() {
		if p := cmd.Process; p != nil {
			p.Kill()
		}
	}

	return cmd, destroy
}

// This is not a real test. This is just a helper process kicked off by tests
// using the helperProcess helper function.
func TestHelperProcess(t *testing.T) {
	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}

		args = args[1:]
	}

	if len(args) == 0 || args[0] != helperProcessSentinel {
		return
	}

	defer os.Exit(0)
	args = args[1:] // strip sentinel value
	cmd := args[0]

	switch cmd {
	case "parent-signal":
		// This subcommand forwards signals to a child process subcommand "print-signal".

		limitProcessLifetime(2 * time.Minute)

		cmd, destroy := helperProcess("print-signal")
		defer destroy()
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "child process failed to start: %v\n", err)
			os.Exit(1)
		}

		doneCh := make(chan struct{})
		defer func() { close(doneCh) }()
		logFn := func(err error) {
			fmt.Fprintf(os.Stderr, "could not forward signal: %s\n", err)
			os.Exit(1)
		}
		ForwardSignals(cmd, logFn, doneCh)

		if err := cmd.Wait(); err != nil {
			fmt.Fprintf(os.Stderr, "unexpected error waiting for child: %v", err)
			os.Exit(1)
		}

	case "print-signal":
		// This subcommand is instrumented to help verify signals are passed correctly.

		limitProcessLifetime(2 * time.Minute)

		ch := make(chan os.Signal, 10)
		signal.Notify(ch, forwardSignals...)
		defer signal.Stop(ch)

		fmt.Fprintf(os.Stdout, "ready\n")

		s := <-ch

		fmt.Fprintf(os.Stdout, "signal: %s\n", s)

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %q\n", cmd)
		os.Exit(2)
	}
}

// limitProcessLifetime installs a background goroutine that self-exits after
// the specified duration elapses to prevent leaking processes from tests that
// may spawn them.
func limitProcessLifetime(dur time.Duration) {
	go time.AfterFunc(dur, func() {
		os.Exit(99)
	})
}

func TestForwardSignals(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, s := range forwardSignals {
		t.Run("signal-"+s.String(), func(t *testing.T) {
			testForwardSignal(t, s)
		})
	}
}

func testForwardSignal(t *testing.T, s os.Signal) {
	t.Helper()

	if s == os.Kill {
		t.Fatalf("you can't forward SIGKILL")
	}

	// Launch a child process which registers the forwarding signal handler
	// under test and then that in turn launches a grand child process that is
	// our test instrument.
	cmd, destroy := helperProcess("parent-signal")
	defer destroy()

	cmd.Stderr = os.Stderr
	prc, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("could not open stdout pipe for child process: %v", err)
	}
	defer prc.Close()

	if err := cmd.Start(); err != nil {
		t.Fatalf("child process failed to start: %v", err)
	}
	scan := bufio.NewScanner(prc)

	// Wait until the grandchild relays back to us that it's ready to receive
	// signals.
	expectLine(t, "ready", scan)

	// Relay our chosen signal down through the intermediary process.
	if err := cmd.Process.Signal(s); err != nil {
		t.Fatalf("signalling child failed: %v", err)
	}

	// Verify that the signal we intended made it all the way to the grandchild.
	expectLine(t, "signal: "+s.String(), scan)
}

func expectLine(t *testing.T, expect string, scan *bufio.Scanner) {
	if !scan.Scan() {
		if scan.Err() != nil {
			t.Fatalf("expected to read line %q but failed: %v", expect, scan.Err())
		} else {
			t.Fatalf("expected to read line %q but got no line", expect)
		}
	}

	if line := scan.Text(); expect != line {
		t.Fatalf("expected to read line %q but got %q", expect, line)
	}
}
