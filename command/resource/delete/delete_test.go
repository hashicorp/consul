// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1
package delete

import (
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/command/resource/apply"
	"github.com/hashicorp/consul/testrpc"
)

func TestResourceDeleteInvalidArgs(t *testing.T) {
	t.Parallel()

	type tc struct {
		args           []string
		expectedCode   int
		expectedErrMsg string
	}

	cases := map[string]tc{
		"nil args": {
			args:           nil,
			expectedCode:   1,
			expectedErrMsg: "Must specify two arguments: resource type and resource name\n",
		},
		"empty args": {
			args:           []string{},
			expectedCode:   1,
			expectedErrMsg: "Must specify two arguments: resource type and resource name\n",
		},
		"missing file path": {
			args:           []string{"-f"},
			expectedCode:   1,
			expectedErrMsg: "Failed to parse args: flag needs an argument: -f",
		},
		"missing resource name": {
			args:           []string{"a.b.c"},
			expectedCode:   1,
			expectedErrMsg: "Must specify two arguments: resource type and resource name",
		},
		"mal-formed group.version.kind": {
			args:           []string{"a.b", "name"},
			expectedCode:   1,
			expectedErrMsg: "Must include resource type argument in group.verion.kind format",
		},
		"does not provide resource name after type": {
			args:           []string{"a.b.c", "-namespace", "default"},
			expectedCode:   1,
			expectedErrMsg: "Must provide resource name right after type",
		},
	}

	for desc, tc := range cases {
		t.Run(desc, func(t *testing.T) {
			ui := cli.NewMockUi()
			c := New(ui)

			require.Equal(t, tc.expectedCode, c.Run(tc.args))
			require.Contains(t, ui.ErrorWriter.String(), tc.expectedErrMsg)
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

func TestResourceDelete(t *testing.T) {
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
	cases := []struct {
		name           string
		args           []string
		expectedCode   int
		errMsg         string
		createResource bool
	}{
		{
			name:           "delete resource in hcl format",
			args:           []string{"-f=../testdata/demo.hcl"},
			expectedCode:   0,
			errMsg:         "",
			createResource: true,
		},
		{
			name:           "delete resource in command line format",
			args:           []string{"demo.v2.Artist", "korn", "-partition=default", "-namespace=default", "-peer=local"},
			expectedCode:   0,
			errMsg:         "",
			createResource: true,
		},
		{
			name:           "delete resource that doesn't exist in command line format",
			args:           []string{"demo.v2.Artist", "fake-korn", "-partition=default", "-namespace=default", "-peer=local"},
			expectedCode:   0,
			errMsg:         "",
			createResource: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ui := cli.NewMockUi()
			c := New(ui)
			cliArgs := append(tc.args, defaultCmdArgs...)
			if tc.createResource {
				createResource(t, a)
			}
			code := c.Run(cliArgs)
			require.Equal(t, ui.ErrorWriter.String(), tc.errMsg)
			require.Equal(t, tc.expectedCode, code)
		})
	}
}
