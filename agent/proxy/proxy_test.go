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

// helperProcess returns an *exec.Cmd that can be used to execute the
// TestHelperProcess function below. This can be used to test multi-process
// interactions.
func helperProcess(s ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--"}
	cs = append(cs, s...)
	env := []string{"GO_WANT_HELPER_PROCESS=1"}

	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = append(env, os.Environ()...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

// This is not a real test. This is just a helper process kicked off by tests
// using the helperProcess helper function.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	defer os.Exit(0)

	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}

		args = args[1:]
	}

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No command\n")
		os.Exit(2)
	}

	cmd, args := args[0], args[1:]
	switch cmd {
	// While running, this creates a file in the given directory (args[0])
	// and deletes it only whe nit is stopped.
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
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %q\n", cmd)
		os.Exit(2)
	}
}
