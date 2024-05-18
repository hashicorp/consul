// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package list

import (
	"errors"
	"testing"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/testrpc"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/command/resource/apply"
)

func TestResourceListCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	applyCli := cli.NewMockUi()

	applyCmd := apply.New(applyCli)
	code := applyCmd.Run([]string{
		"-f=../testdata/demo.hcl",
		"-http-addr=" + a.HTTPAddr(),
		"-token=root",
	})
	require.Equal(t, 0, code)
	require.Empty(t, applyCli.ErrorWriter.String())
	require.Contains(t, applyCli.OutputWriter.String(), "demo.v2.Artist 'korn' created.")

	cases := []struct {
		name      string
		output    string
		extraArgs []string
	}{
		{
			name:   "sample output",
			output: "\"name\": \"korn\"",
			extraArgs: []string{
				"demo.v2.artist",
				"-namespace=default",
				"-partition=default",
			},
		},
		{
			name:   "file input",
			output: "\"name\": \"korn\"",
			extraArgs: []string{
				"-f=../testdata/demo.hcl",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ui := cli.NewMockUi()
			c := New(ui)

			args := []string{
				"-http-addr=" + a.HTTPAddr(),
				"-token=root",
			}

			args = append(args, tc.extraArgs...)

			actualCode := c.Run(args)
			require.Equal(t, 0, actualCode)
			require.Empty(t, ui.ErrorWriter.String())
			require.Contains(t, ui.OutputWriter.String(), tc.output)
		})
	}
}

func TestResourceListInvalidArgs(t *testing.T) {
	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	type tc struct {
		args         []string
		expectedCode int
		expectedErr  error
	}

	cases := map[string]tc{
		"nil args": {
			args:         nil,
			expectedCode: 1,
			expectedErr:  errors.New("Incorrect argument format: Must include resource type argument"),
		},
		"minimum args required": {
			args:         []string{},
			expectedCode: 1,
			expectedErr:  errors.New("Incorrect argument format: Must include resource type argument"),
		},
		"no file path": {
			args: []string{
				"-f",
			},
			expectedCode: 1,
			expectedErr:  errors.New("Failed to parse args: flag needs an argument: -f"),
		},
		"file not found": {
			args: []string{
				"-f=../testdata/test.hcl",
			},
			expectedCode: 1,
			expectedErr:  errors.New("Failed to load data: Failed to read file: open ../testdata/test.hcl: no such file or directory"),
		},
		"file parsing failure": {
			args: []string{
				"-f=../testdata/invalid_type.hcl",
			},
			expectedCode: 1,
			expectedErr:  errors.New("Failed to decode resource from input file"),
		},
		"file argument with resource type": {
			args: []string{
				"demo.v2.artist",
				"-namespace=default",
				"-partition=default",
				"-http-addr=" + a.HTTPAddr(),
				"-token=root",
				"-f=demo.hcl",
			},
			expectedCode: 1,
			expectedErr:  errors.New("Incorrect argument format: File argument is not needed when resource information is provided with the command"),
		},
		"resource type invalid": {
			args: []string{
				"test",
				"-namespace=default",
				"-partition=default",
			},
			expectedCode: 1,
			expectedErr:  errors.New("Must include resource type argument in group.version.kind format"),
		},
		"resource name is provided": {
			args: []string{
				"demo.v2.artist",
				"test",
				"-namespace=default",
				"-partition=default",
			},
			expectedCode: 1,
			expectedErr:  errors.New("Must include flag arguments after resource type"),
		},
	}

	for desc, tc := range cases {
		t.Run(desc, func(t *testing.T) {
			ui := cli.NewMockUi()
			c := New(ui)

			code := c.Run(tc.args)

			require.Equal(t, tc.expectedCode, code)
			require.Contains(t, ui.ErrorWriter.String(), tc.expectedErr.Error())
		})
	}
}
