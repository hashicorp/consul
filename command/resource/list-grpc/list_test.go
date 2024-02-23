// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package list

import (
	"errors"
	"fmt"
	"testing"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/sdk/freeport"
	"github.com/hashicorp/consul/testrpc"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/command/resource/apply-grpc"
)

func TestResourceListCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	availablePort := freeport.GetOne(t)
	a := agent.NewTestAgent(t, fmt.Sprintf("ports { grpc = %d }", availablePort))
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	t.Cleanup(func() {
		a.Shutdown()
	})

	applyCli := cli.NewMockUi()

	applyCmd := apply.New(applyCli)
	code := applyCmd.Run([]string{
		"-f=../testdata/demo.hcl",
		fmt.Sprintf("-grpc-addr=127.0.0.1:%d", availablePort),
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
				"demo.v2.Artist",
				"-partition=default",
				"-namespace=default",
			},
		},
		{
			name:   "sample output with name prefix",
			output: "\"name\": \"korn\"",
			extraArgs: []string{
				"demo.v2.Artist",
				"-p=korn",
				"-partition=default",
				"-namespace=default",
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
				fmt.Sprintf("-grpc-addr=127.0.0.1:%d", availablePort),
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

	availablePort := freeport.GetOne(t)
	a := agent.NewTestAgent(t, fmt.Sprintf("ports { grpc = %d }", availablePort))
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	t.Cleanup(func() {
		a.Shutdown()
	})

	type tc struct {
		args         []string
		expectedCode int
		expectedErr  error
	}

	cases := map[string]tc{
		"nil args": {
			args:         nil,
			expectedCode: 1,
			expectedErr:  errors.New("Incorrect argument format: Must include resource type or flag arguments"),
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
				"demo.v2.Artist",
				"-partition=default",
				"-namespace=default",
				fmt.Sprintf("-grpc-addr=127.0.0.1:%d", availablePort),
				"-token=root",
				"-f=demo.hcl",
			},
			expectedCode: 1,
			expectedErr:  errors.New("Incorrect argument format: File argument is not needed when resource information is provided with the command"),
		},
		"resource type invalid": {
			args: []string{
				"test",
				"-partition=default",
				"-namespace=default",
			},
			expectedCode: 1,
			expectedErr:  errors.New("Incorrect argument format: The shorthand name does not map to any existing resource type"),
		},
		"resource name is provided": {
			args: []string{
				"demo.v2.Artist",
				"test",
				"-namespace=default",
				"-partition=default",
			},
			expectedCode: 1,
			expectedErr:  errors.New("Incorrect argument format: Must include flag arguments after resource type"),
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
