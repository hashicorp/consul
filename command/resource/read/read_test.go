// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1
package read

import (
	"errors"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/command/resource/apply"
	"github.com/hashicorp/consul/testrpc"
)

func TestResourceReadInvalidArgs(t *testing.T) {
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
			expectedErr:  errors.New("Incorrect argument format: Must provide resource type argument with either in group.version.kind format or its shorthand name"),
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

func createResource(t *testing.T, a *agent.TestAgent) {
	applyUi := cli.NewMockUi()
	applyCmd := apply.New(applyUi)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-token=root",
	}

	args = append(args, []string{"-f=../testdata/demo.hcl"}...)

	code := applyCmd.Run(args)
	require.Equal(t, 0, code)
	require.Empty(t, applyUi.ErrorWriter.String())
}

func TestResourceRead(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	defaultCmdArgs := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-token=root",
	}

	createResource(t, a)
	cases := []struct {
		name         string
		args         []string
		expectedCode int
		errMsg       string
	}{
		{
			name:         "read resource in hcl format",
			args:         []string{"-f=../testdata/demo.hcl"},
			expectedCode: 0,
			errMsg:       "",
		},
		{
			name:         "read resource in command line format",
			args:         []string{"demo.v2.Artist", "korn", "-partition=default", "-namespace=default"},
			expectedCode: 0,
			errMsg:       "",
		},
		{
			name:         "read resource that doesn't exist",
			args:         []string{"demo.v2.Artist", "fake-korn", "-partition=default", "-namespace=default"},
			expectedCode: 1,
			errMsg:       "Error reading resource demo.v2.Artist/fake-korn: Unexpected response code: 404 (rpc error: code = NotFound desc = resource not found)\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ui := cli.NewMockUi()
			c := New(ui)
			cliArgs := append(tc.args, defaultCmdArgs...)
			code := c.Run(cliArgs)
			require.Equal(t, tc.errMsg, ui.ErrorWriter.String())
			require.Equal(t, tc.expectedCode, code)
		})
	}
}
