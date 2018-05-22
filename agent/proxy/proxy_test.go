package proxy

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
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

// helperProcessDaemon returns a *Daemon that can be used to execute the
// TestHelperProcess function below. This can be used to test multi-process
// interactions. The Daemon has it's Path, Args and stdio paths populated but
// other fields might need to be set depending on test requirements.
//
// NOTE: this relies on a sufficiently recent version on consul being installed
// in your path so that the daemonize command can be used. That's gross but hard
// to see how we can do better given that tests are separate binaries and we
// need consul's daemonize mode to work correctly. I considered hacks around
// building the local tree and getting the absolute path to the resulting binary
// but that seems gross in a different way. This is the same or weaker
// assumption our `api` test suit makes already...
func helperProcessDaemon(s ...string) *Daemon {
	cs := []string{os.Args[0], "-test.run=TestHelperProcess", "--", helperProcessSentinel}
	cs = append(cs, s...)

	path, err := exec.LookPath("consul")
	if err != nil || path == "" {
		panic("consul not found on $PATH - download and install " +
			"consul or skip this test")
	}

	return &Daemon{
		Path:          os.Args[0],
		Args:          cs,
		StdoutPath:    "_", // dev null them for now
		StderrPath:    "_",
		daemonizePath: path,
	}
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
		var data []byte
		data = append(data, []byte(os.Getenv(EnvProxyId))...)
		data = append(data, ':')
		data = append(data, []byte(os.Getenv(EnvProxyToken))...)

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

	case "output":
		fmt.Fprintf(os.Stdout, "hello stdout\n")
		fmt.Fprintf(os.Stderr, "hello stderr\n")

		// Sync to be sure it is written out of buffers
		os.Stdout.Sync()
		os.Stderr.Sync()

		// Output a file to signal we've written to stdout/err
		path := args[0]
		if err := ioutil.WriteFile(path, []byte("hello"), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}

		<-make(chan struct{})

	// Parent runs the given process in a Daemon and then exits. It exists to test
	// that the Daemon-managed child process survives it's parent exiting which we
	// can't test directly without exiting the test process so we need an extra
	// level of indirection. The caller must pass a file path as the first
	// argument for the child processes PID to be written and then must take care
	// to clean up that PID later or the child will be left running forever.
	case "parent":
		// We will write the PID for the child to the file in the first argument
		// then pass rest of args through to command.
		pidFile := args[0]
		d := helperProcess(args[1:]...)
		if err := d.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}
		// Write PID
		pidBs := []byte(strconv.Itoa(d.Process.Pid))
		if err := ioutil.WriteFile(pidFile, pidBs, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}
		// Wait "forever" (calling test chooses when we exit with signal/Wait to
		// minimise coordination)
		for {
			time.Sleep(time.Hour)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %q\n", cmd)
		os.Exit(2)
	}
}
