// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1
package delete

import (
	"errors"
	"fmt"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/command/resource/apply-grpc"
	"github.com/hashicorp/consul/sdk/freeport"
	"github.com/hashicorp/consul/testrpc"
)

func TestResourceDeleteInvalidArgs(t *testing.T) {
	t.Parallel()

	type tc struct {
		args         []string
		expectedCode int
		expectedErr  error
	}

	cases := map[string]tc{
		"nil args": {
			args:         nil,
			expectedCode: 1,
			expectedErr:  errors.New("Incorrect argument format: Must specify two arguments: resource type and resource name"),
		},
		"empty args": {
			args:         []string{},
			expectedCode: 1,
			expectedErr:  errors.New("Incorrect argument format: Must specify two arguments: resource type and resource name"),
		},
		"missing file path": {
			args:         []string{"-f"},
			expectedCode: 1,
			expectedErr:  errors.New("Failed to parse args: flag needs an argument: -f"),
		},
		"file not found": {
			args:         []string{"-f=../testdata/test.hcl"},
			expectedCode: 1,
			expectedErr:  errors.New("Failed to load data: Failed to read file: open ../testdata/test.hcl: no such file or directory"),
		},
		"provide type and name": {
			args:         []string{"a.b.c"},
			expectedCode: 1,
			expectedErr:  errors.New("Incorrect argument format: Must specify two arguments: resource type and resource name"),
		},
		"provide type and name with -f": {
			args:         []string{"a.b.c", "name", "-f", "test.hcl"},
			expectedCode: 1,
			expectedErr:  errors.New("Incorrect argument format: File argument is not needed when resource information is provided with the command"),
		},
		"provide type and name with -f and other flags": {
			args:         []string{"a.b.c", "name", "-f", "test.hcl", "-namespace", "default"},
			expectedCode: 1,
			expectedErr:  errors.New("Incorrect argument format: File argument is not needed when resource information is provided with the command"),
		},
		"does not provide resource name after type": {
			args:         []string{"a.b.c", "-namespace", "default"},
			expectedCode: 1,
			expectedErr:  errors.New("Incorrect argument format: Must provide resource name right after type"),
		},
		"invalid resource type format": {
			args:         []string{"a.", "name", "-namespace", "default"},
			expectedCode: 1,
			expectedErr:  errors.New("Must provide resource type argument with either in group.version.kind format or its shorthand name"),
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

func createResource(t *testing.T, port int) {
	applyUi := cli.NewMockUi()
	applyCmd := apply.New(applyUi)

	args := []string{
		fmt.Sprintf("-grpc-addr=127.0.0.1:%d", port),
		"-token=root",
	}

	args = append(args, []string{"-f=../testdata/demo.hcl"}...)

	code := applyCmd.Run(args)
	require.Equal(t, 0, code)
	require.Empty(t, applyUi.ErrorWriter.String())
}

func TestResourceDelete(t *testing.T) {
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

	defaultCmdArgs := []string{
		fmt.Sprintf("-grpc-addr=127.0.0.1:%d", availablePort),
		"-token=root",
	}

	cases := []struct {
		name           string
		args           []string
		expectedCode   int
		createResource bool
	}{
		{
			name:           "delete resource in hcl format",
			args:           []string{"-f=../testdata/demo.hcl"},
			expectedCode:   0,
			createResource: true,
		},
		{
			name:           "delete resource in command line format",
			args:           []string{"demo.v2.Artist", "korn", "-partition=default", "-namespace=default"},
			expectedCode:   0,
			createResource: true,
		},
		{
			name:           "delete resource that doesn't exist in command line format",
			args:           []string{"demo.v2.Artist", "korn", "-partition=default", "-namespace=default"},
			expectedCode:   0,
			createResource: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ui := cli.NewMockUi()
			c := New(ui)
			cliArgs := append(tc.args, defaultCmdArgs...)
			if tc.createResource {
				createResource(t, availablePort)
			}
			code := c.Run(cliArgs)
			require.Empty(t, ui.ErrorWriter.String())
			require.Equal(t, tc.expectedCode, code)
			require.Contains(t, ui.OutputWriter.String(), "deleted")
		})
	}
}
