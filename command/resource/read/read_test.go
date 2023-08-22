// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1
package read

import (
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestResourceReadInvalidArgs(t *testing.T) {
	t.Parallel()

	type tc struct {
		args           []string
		expectedCode   int
		expectedErrMsg string
	}

	cases := map[string]tc{
		"missing file path": {
			args:           []string{"-f"},
			expectedCode:   1,
			expectedErrMsg: "Please input file path",
		},
		"provide type and name": {
			args:           []string{"a.b.c"},
			expectedCode:   1,
			expectedErrMsg: "Must specify two arguments: resource type and resource name",
		},
		"provide type and name with -f": {
			args:           []string{"a.b.c", "name", "-f", "test.hcl"},
			expectedCode:   1,
			expectedErrMsg: "You need to provide all information in the HCL file if provide its file path",
		},
		"provide type and name with -f and other flags": {
			args:           []string{"a.b.c", "name", "-f", "test.hcl", "-namespace", "default"},
			expectedCode:   1,
			expectedErrMsg: "You need to provide all information in the HCL file if provide its file path",
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
			require.NotEmpty(t, ui.ErrorWriter.String())
		})
	}
}

func TestResourceRead(t *testing.T) {
	// TODO: add read test after apply checked in
	//if testing.Short() {
	//	t.Skip("too slow for testing.Short")
	//}
	//
	//t.Parallel()
	//
	//a := agent.NewTestAgent(t, ``)
	//defer a.Shutdown()
	//client := a.Client()
	//
	//ui := cli.NewMockUi()
	//c := New(ui)

	//_, _, err := client.Resource().Apply()
	//require.NoError(t, err)
	//
	//args := []string{}
	//
	//code := c.Run(args)
	//require.Equal(t, 0, code)
}
