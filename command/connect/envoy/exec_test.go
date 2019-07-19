// +build linux darwin

package envoy

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestExecEnvoy(t *testing.T) {

	cases := []struct {
		Name     string
		Args     []string
		WantArgs []string
	}{
		{
			Name: "default",
			Args: []string{},
			WantArgs: []string{
				"--config-path",
				"{{ got.ConfigPath }}",
				"--disable-hot-restart",
				"--fake-envoy-arg",
			},
		},
		{
			Name: "hot-restart-epoch",
			Args: []string{"--restart-epoch", "1"},
			WantArgs: []string{
				"--config-path",
				// Different platforms produce different file descriptors here so we use the
				// value we got back. This is somewhat tautological but we do sanity check
				// that value further below.
				"{{ got.ConfigPath }}",
				// No --disable-hot-restart
				"--fake-envoy-arg",
				"--restart-epoch",
				"1",
			},
		},
		{
			Name: "hot-restart-version",
			Args: []string{"--drain-time-s", "10"},
			WantArgs: []string{
				"--config-path",
				// Different platforms produce different file descriptors here so we use the
				// value we got back. This is somewhat tautological but we do sanity check
				// that value further below.
				"{{ got.ConfigPath }}",
				// No --disable-hot-restart
				"--fake-envoy-arg",
				// Restart epoch defaults to 0 if not given and not disabled.
				"--drain-time-s",
				"10",
			},
		},
		{
			Name: "hot-restart-version",
			Args: []string{"--parent-shutdown-time-s", "20"},
			WantArgs: []string{
				"--config-path",
				// Different platforms produce different file descriptors here so we use the
				// value we got back. This is somewhat tautological but we do sanity check
				// that value further below.
				"{{ got.ConfigPath }}",
				// No --disable-hot-restart
				"--fake-envoy-arg",
				// Restart epoch defaults to 0 if not given and not disabled.
				"--parent-shutdown-time-s",
				"20",
			},
		},
		{
			Name: "hot-restart-version",
			Args: []string{"--hot-restart-version", "foobar1"},
			WantArgs: []string{
				"--config-path",
				// Different platforms produce different file descriptors here so we use the
				// value we got back. This is somewhat tautological but we do sanity check
				// that value further below.
				"{{ got.ConfigPath }}",
				// No --disable-hot-restart
				"--fake-envoy-arg",
				// Restart epoch defaults to 0 if not given and not disabled.
				"--hot-restart-version",
				"foobar1",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			require := require.New(t)

			args := append([]string{"exec-fake-envoy"}, tc.Args...)
			cmd, destroy := helperProcess(args...)
			defer destroy()

			cmd.Stderr = os.Stderr
			outBytes, err := cmd.Output()
			require.NoError(err)

			var got FakeEnvoyExecData
			require.NoError(json.Unmarshal(outBytes, &got))

			expectConfigData := fakeEnvoyTestData

			// Substitute the right FD path
			for idx := range tc.WantArgs {
				tc.WantArgs[idx] = strings.Replace(tc.WantArgs[idx],
					"{{ got.ConfigPath }}", got.ConfigPath, 1)
			}

			require.Equal(tc.WantArgs, got.Args)
			require.Equal(expectConfigData, got.ConfigData)
			// Sanity check the config path in a non-brittle way since we used it to
			// generate expectation for the args.
			require.Regexp(`-bootstrap.json$`, got.ConfigPath)
		})
	}
}

type FakeEnvoyExecData struct {
	Args       []string `json:"args"`
	ConfigPath string   `json:"configPath"`
	ConfigData string   `json:"configData"`
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

const fakeEnvoyTestData = "pahx9eiPoogheb4haeb2abeem1QuireWahtah1Udi5ae4fuD0c"

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
	case "exec-fake-envoy":
		// this will just exec the "fake-envoy" flavor below

		limitProcessLifetime(2 * time.Minute)

		// This is another level of gross - we are relying on `consul` being on path
		// and being the correct version but in general that is true under `make
		// test`. We already make the same assumption for API package tests.
		testSelfExecOverride = "consul"

		err := execEnvoy(
			os.Args[0],
			[]string{
				"-test.run=TestHelperProcess",
				"--",
				helperProcessSentinel,
				"fake-envoy",
			},
			append([]string{"--fake-envoy-arg"}, args...),
			[]byte(fakeEnvoyTestData),
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "fake envoy process failed to exec: %v\n", err)
			os.Exit(1)
		}

	case "fake-envoy":
		// This subcommand is instrumented to verify some settings
		// survived an exec.

		limitProcessLifetime(2 * time.Minute)

		data := FakeEnvoyExecData{
			Args: args,
		}

		// Dump all of the args.
		var captureNext bool
		for _, arg := range args {
			if arg == "--config-path" {
				captureNext = true
			} else if captureNext {
				data.ConfigPath = arg
				captureNext = false
			}
		}

		if data.ConfigPath == "" {
			fmt.Fprintf(os.Stderr, "did not detect a --config-path argument passed through\n")
			os.Exit(1)
		}

		d, err := ioutil.ReadFile(data.ConfigPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not read provided --config-path file %q: %v\n", data.ConfigPath, err)
			os.Exit(1)
		}
		data.ConfigData = string(d)

		enc := json.NewEncoder(os.Stdout)
		if err := enc.Encode(&data); err != nil {
			fmt.Fprintf(os.Stderr, "could not dump results to stdout: %v", err)
			os.Exit(1)

		}

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
