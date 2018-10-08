package envoy

import (
	"flag"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent/xds"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "update golden files")

func TestCatalogCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(nil).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

// testSetAndResetEnv sets the env vars passed as KEY=value strings in the
// current ENV and returns a func() that will undo it's work at the end of the
// test for use with defer.
func testSetAndResetEnv(t *testing.T, env []string) func() {
	old := make(map[string]*string)
	for _, e := range env {
		pair := strings.SplitN(e, "=", 2)
		current := os.Getenv(pair[0])
		if current != "" {
			old[pair[0]] = &current
		} else {
			// save it as a nil so we know to remove again
			old[pair[0]] = nil
		}
		os.Setenv(pair[0], pair[1])
	}
	// Return a func that will reset to old values
	return func() {
		for k, v := range old {
			if v == nil {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, *v)
			}
		}
	}
}

// This tests the args we use to generate the template directly because they
// encapsulate all the argument and default handling code which is where most of
// the logic is. We also allow generating golden files but only for cases that
// pass the test of having their template args generated as expected.
func TestGenerateConfig(t *testing.T) {
	cases := []struct {
		Name     string
		Flags    []string
		Env      []string
		WantArgs templateArgs
		WantErr  string
	}{
		{
			Name:    "no-args",
			Flags:   []string{},
			Env:     []string{},
			WantErr: "No proxy ID specified",
		},
		{
			Name:  "defaults",
			Flags: []string{"-proxy-id", "test-proxy"},
			Env:   []string{},
			WantArgs: templateArgs{
				ProxyCluster:          "test-proxy",
				ProxyID:               "test-proxy",
				AgentAddress:          "127.0.0.1",
				AgentPort:             "8502", // Note this is the gRPC port
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
			},
		},
		{
			Name: "grpc-addr-flag",
			Flags: []string{"-proxy-id", "test-proxy",
				"-grpc-addr", "localhost:9999"},
			Env: []string{},
			WantArgs: templateArgs{
				ProxyCluster: "test-proxy",
				ProxyID:      "test-proxy",
				// Should resolve IP, note this might not resolve the same way
				// everywhere which might make this test brittle but not sure what else
				// to do.
				AgentAddress:          "127.0.0.1",
				AgentPort:             "9999",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
			},
		},
		{
			Name:  "grpc-addr-env",
			Flags: []string{"-proxy-id", "test-proxy"},
			Env: []string{
				"CONSUL_GRPC_ADDR=localhost:9999",
			},
			WantArgs: templateArgs{
				ProxyCluster: "test-proxy",
				ProxyID:      "test-proxy",
				// Should resolve IP, note this might not resolve the same way
				// everywhere which might make this test brittle but not sure what else
				// to do.
				AgentAddress:          "127.0.0.1",
				AgentPort:             "9999",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
			},
		},
		// TODO(banks): all the flags/env manipulation cases
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			require := require.New(t)

			ui := cli.NewMockUi()
			c := New(ui)

			defer testSetAndResetEnv(t, tc.Env)()

			// Run the command
			args := append([]string{"-bootstrap"}, tc.Flags...)
			code := c.Run(args)
			if tc.WantErr == "" {
				require.Equal(0, code, ui.ErrorWriter.String())
			} else {
				require.Equal(1, code, ui.ErrorWriter.String())
				require.Contains(ui.ErrorWriter.String(), tc.WantErr)
				return
			}

			// Verify we handled the env and flags right first to get correct template
			// args.
			got, err := c.templateArgs()
			require.NoError(err) // Error cases should have returned above
			require.Equal(&tc.WantArgs, got)

			// Actual template output goes to stdout direct to avoid prefix in UI, so
			// generate it again here to assert on.
			actual, err := c.generateConfig()
			require.NoError(err)

			// If we got the arg handling write, verify output
			golden := filepath.Join("testdata", tc.Name+".golden")
			if *update {
				ioutil.WriteFile(golden, actual, 0644)
			}

			expected, err := ioutil.ReadFile(golden)
			require.NoError(err)
			require.Equal(string(expected), string(actual))
		})
	}
}
