// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package apply

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/sdk/freeport"
	"github.com/hashicorp/consul/testrpc"
)

func TestResourceApplyCommand(t *testing.T) {
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
			output: "demo.v2.Festival 'woodstock' created.",
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

			args = append(args, tc.args...)

			code := c.Run(args)
			require.Equal(t, 0, code)
			require.Empty(t, ui.ErrorWriter.String())
			require.Contains(t, ui.OutputWriter.String(), tc.output)
		})
	}
}

func TestResourceApplyCommand_StdIn(t *testing.T) {
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

	t.Run("hcl", func(t *testing.T) {
		stdinR, stdinW := io.Pipe()

		ui := cli.NewMockUi()
		c := New(ui)
		c.testStdin = stdinR

		stdInput := `ID {
			Type = gvk("demo.v2.Artist")
			Name = "korn"
			Tenancy {
			  Partition = "default"
			  Namespace = "default"
			}
		  }
		  
		  Data {
			Name = "Korn"
			Genre = "GENRE_METAL"
		  }
		  
		  Metadata = {
			"foo" = "bar"
		  }`

		go func() {
			stdinW.Write([]byte(stdInput))
			stdinW.Close()
		}()

		args := []string{
			fmt.Sprintf("-grpc-addr=127.0.0.1:%d", availablePort),
			"-token=root",
			"-f",
			"-",
		}

		code := c.Run(args)
		require.Equal(t, 0, code)
		require.Empty(t, ui.ErrorWriter.String())
		// Todo: make up the read result check after finishing the read command
		//expected := readResource(t, a, []string{"demo.v2.Artist", "korn"})
		require.Contains(t, ui.OutputWriter.String(), "demo.v2.Artist 'korn' created.")
		//require.Contains(t, ui.OutputWriter.String(), expected)
	})

	t.Run("json", func(t *testing.T) {
		stdinR, stdinW := io.Pipe()

		ui := cli.NewMockUi()
		c := New(ui)
		c.testStdin = stdinR

		stdInput := `{
			"data": {
				"genre": "GENRE_METAL",
				"name": "Korn"
			},
			"id": {
				"name": "korn",
				"tenancy": {
					"partition": "default",
					"namespace": "default"
				},
				"type": {
					"group": "demo",
					"groupVersion": "v2",
					"kind": "Artist"
				}
			},
			"metadata": {
				"foo": "bar"
			}
		}`

		go func() {
			stdinW.Write([]byte(stdInput))
			stdinW.Close()
		}()

		args := []string{
			"-f",
			"-",
			fmt.Sprintf("-grpc-addr=127.0.0.1:%d", availablePort),
			"-token=root",
		}

		code := c.Run(args)
		require.Equal(t, 0, code)
		require.Empty(t, ui.ErrorWriter.String())
		// Todo: make up the read result check after finishing the read command
		//expected := readResource(t, a, []string{"demo.v2.Artist", "korn"})
		require.Contains(t, ui.OutputWriter.String(), "demo.v2.Artist 'korn' created.")
		//require.Contains(t, ui.OutputWriter.String(), expected)
	})
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
			expectedErr:  errors.New("Required '-f' flag was not provided to specify where to load the resource content from"),
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
