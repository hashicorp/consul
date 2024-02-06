// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/internal/testing/golden"
	"github.com/hashicorp/consul/testrpc"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

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
			input, err := os.ReadFile(statePath)
			require.NoError(t, err)

			var state api.AutopilotState
			require.NoError(t, json.Unmarshal(input, &state))

			for _, format := range GetSupportedFormats() {
				t.Run(format, func(t *testing.T) {
					formatter, err := NewFormatter(format)
					require.NoError(t, err)

					actual, err := formatter.FormatState(&state)
					require.NoError(t, err)

					expected := golden.Get(t, actual, filepath.Join(name, format))
					require.Equal(t, expected, actual)
				})
			}
		})
	}
}
