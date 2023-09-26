// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package apply

import (
	"errors"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/testrpc"
)

func TestResourceApplyCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	cases := []struct {
		name   string
		output string
		args   []string
	}{
		{
			name:   "sample output",
			args:   []string{"-f=../testdata/demo.hcl"},
			output: "demo.v2.Artist 'korn' created.",
		},
		{
			name:   "nested data format",
			args:   []string{"-f=../testdata/nested_data.hcl"},
			output: "mesh.v2beta1.Destinations 'api' created.",
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

			args = append(args, tc.args...)

			code := c.Run(args)
			require.Equal(t, 0, code)
			require.Empty(t, ui.ErrorWriter.String())
			require.Contains(t, ui.OutputWriter.String(), tc.output)
		})
	}
}

func TestResourceApplyInvalidArgs(t *testing.T) {
	t.Parallel()

	type tc struct {
		args         []string
		expectedCode int
		expectedErr  error
	}

	cases := map[string]tc{
		"no file path": {
			args:         []string{"-f"},
			expectedCode: 1,
			expectedErr:  errors.New("Failed to parse args: flag needs an argument: -f"),
		},
		"missing required flag": {
			args:         []string{},
			expectedCode: 1,
			expectedErr:  errors.New("Incorrect argument format: Flag -f with file path argument is required"),
		},
		"file parsing failure": {
			args:         []string{"-f=../testdata/invalid.hcl"},
			expectedCode: 1,
			expectedErr:  errors.New("Failed to decode resource from input file"),
		},
		"file not found": {
			args:         []string{"-f=../testdata/test.hcl"},
			expectedCode: 1,
			expectedErr:  errors.New("Failed to load data: Failed to read file: open ../testdata/test.hcl: no such file or directory"),
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
