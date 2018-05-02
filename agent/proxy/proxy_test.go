package proxy

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"testing"
	"time"
)

// testLogger is a logger that can be used by tests that require a
// *log.Logger instance.
var testLogger = log.New(os.Stderr, "logger: ", log.LstdFlags)

// testTempDir returns a temporary directory and a cleanup function.
func testTempDir(t *testing.T) (string, func()) {
	t.Helper()

	td, err := ioutil.TempDir("", "test-agent-proxy")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	return td, func() {
		if err := os.RemoveAll(td); err != nil {
			t.Fatalf("err: %s", err)
		}
	}
}

// helperProcessSentinel is a sentinel value that is put as the first
// argument following "--" and is used to determine if TestHelperProcess
// should run.
const helperProcessSentinel = "WANT_HELPER_PROCESS"

// helperProcess returns an *exec.Cmd that can be used to execute the
// TestHelperProcess function below. This can be used to test multi-process
// interactions.
func helperProcess(s ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", helperProcessSentinel}
	cs = append(cs, s...)

	cmd := exec.Command(os.Args[0], cs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
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
	cmd, args := args[0], args[1:]
	switch cmd {
	// While running, this creates a file in the given directory (args[0])
	// and deletes it only when it is stopped.
	case "start-stop":
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt)
		defer signal.Stop(ch)

		path := args[0]
		data := []byte(os.Getenv(EnvProxyToken))

		if err := ioutil.WriteFile(path, data, 0644); err != nil {
			t.Fatalf("err: %s", err)
		}
		defer os.Remove(path)

		<-ch

	// Restart writes to a file and keeps running while that file still
	// exists. When that file is removed, this process exits. This can be
	// used to test restarting.
	case "restart":
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt)
		defer signal.Stop(ch)

		// Write the file
		path := args[0]
		if err := ioutil.WriteFile(path, []byte("hello"), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}

		// While the file still exists, do nothing. When the file no longer
		// exists, we exit.
		for {
			time.Sleep(25 * time.Millisecond)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				break
			}

			select {
			case <-ch:
				// We received an interrupt, clean exit
				os.Remove(path)
				break

			default:
			}
		}

	case "stop-kill":
		// Setup listeners so it is ignored
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt)
		defer signal.Stop(ch)

		path := args[0]
		data := []byte(os.Getenv(EnvProxyToken))
		for {
			if err := ioutil.WriteFile(path, data, 0644); err != nil {
				t.Fatalf("err: %s", err)
			}
			time.Sleep(25 * time.Millisecond)
		}

		// Run forever
		<-make(chan struct{})

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %q\n", cmd)
		os.Exit(2)
	}
}
