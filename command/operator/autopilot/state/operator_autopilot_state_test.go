package state

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

// update allows golden files to be updated based on the current output.
var update = flag.Bool("update", false, "update golden files")

// golden reads and optionally writes the expected data to the golden file,
// returning the contents as a string.
func golden(t *testing.T, name, got string) string {
	t.Helper()

	golden := filepath.Join("testdata", name+".golden")
	if *update && got != "" {
		err := ioutil.WriteFile(golden, []byte(got), 0644)
		require.NoError(t, err)
	}

	expected, err := ioutil.ReadFile(golden)
	require.NoError(t, err)

	return string(expected)
}

func TestStateCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestStateCommand_Pretty(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, `
		node_id = "f0427127-7531-455a-b651-f1ea1d8451f0"
	`)

	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	cmd := New(ui)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
	}

	code := cmd.Run(args)
	require.Empty(t, ui.ErrorWriter.String())
	require.Equal(t, code, 0)
	output := ui.OutputWriter.String()

	// Just a few quick checks to ensure we got output
	// the output formatter will be tested in another test.
	require.Regexp(t, `^Healthy:`, output)
	require.Regexp(t, `(?m)^Leader:`, output)
}

func TestStateCommand_JSON(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, "")

	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	cmd := New(ui)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-format=json",
	}

	code := cmd.Run(args)
	require.Empty(t, ui.ErrorWriter.String())
	require.Equal(t, code, 0)
	output := ui.OutputWriter.String()

	var state api.AutopilotState
	require.NoError(t, json.Unmarshal([]byte(output), &state))
}

func TestStateCommand_Formatter(t *testing.T) {
	cases := []string{
		"ce",
		"enterprise",
	}

	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			statePath := filepath.Join("testdata", name, "state.json")
			input, err := ioutil.ReadFile(statePath)
			require.NoError(t, err)

			var state api.AutopilotState
			require.NoError(t, json.Unmarshal(input, &state))

			for _, format := range GetSupportedFormats() {
				t.Run(format, func(t *testing.T) {
					formatter, err := NewFormatter(format)
					require.NoError(t, err)

					actual, err := formatter.FormatState(&state)
					require.NoError(t, err)

					expected := golden(t, filepath.Join(name, format), actual)
					require.Equal(t, expected, actual)
				})
			}
		})
	}
}
