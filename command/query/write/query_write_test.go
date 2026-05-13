// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package write

import (
	"fmt"
	"io"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestQueryWrite_noTabs(t *testing.T) {
	t.Parallel()

	require.NotContains(t, New(cli.NewMockUi()).Help(), "\t")
}

func TestQueryWrite(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	t.Run("File create and update", func(t *testing.T) {
		ui := cli.NewMockUi()
		c := New(ui)

		f := testutil.TempFile(t, "example-query.json")
		_, err := f.WriteString(`{
		  "Name": "example-query-via-file",
		  "Service": {
			"Service": "example-service-via-file",
			"OnlyPassing": true
		  },
		  "DNS": {
			"TTL": "60s"
		  }
	    }`)

		require.NoError(t, err)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			f.Name(),
		}

		code := c.Run(args)
		output := ui.OutputWriter.String()
		stderr := ui.ErrorWriter.String()
		require.Empty(t, stderr)
		require.Contains(t, output, `Query Created: `)
		require.Equal(t, 0, code)

		ID := ""
		fmt.Sscanf(output, "Query Created: %s", &ID)
		queries, _, err := client.PreparedQuery().Get(ID, nil)
		require.NoError(t, err)
		require.Equal(t, 1, len(queries))
		require.Equal(t, "example-query-via-file", queries[0].Name)
		require.Equal(t, "example-service-via-file", queries[0].Service.Service)

		f = testutil.TempFile(t, "example-query.json")
		_, err = f.WriteString(`{
		  "ID": "` + ID + `",
		  "Name": "example-query-via-file",
		  "Service": {
			"Service": "example-service-via-file2",
			"OnlyPassing": true
		  },
		  "DNS": {
			"TTL": "60s"
		  }
	    }`)

		require.NoError(t, err)

		args = []string{
			"-http-addr=" + a.HTTPAddr(),
			f.Name(),
		}

		code = c.Run(args)
		output = ui.OutputWriter.String()
		stderr = ui.ErrorWriter.String()
		require.Empty(t, stderr)
		require.Contains(t, output, `Query Updated: `)
		require.Equal(t, 0, code)

		fmt.Sscanf(output, "Query Updated: %s", &ID)
		queries, _, err = client.PreparedQuery().Get(ID, nil)
		require.NoError(t, err)
		require.Equal(t, 1, len(queries))
		require.Equal(t, "example-query-via-file", queries[0].Name)
		require.Equal(t, "example-service-via-file2", queries[0].Service.Service)
	})

	t.Run("Stdin create and update", func(t *testing.T) {
		stdinR, stdinW := io.Pipe()

		ui := cli.NewMockUi()
		c := New(ui)
		c.testStdin = stdinR

		go func() {
			stdinW.Write([]byte(`{
			"Name": "example-query-via-stdin",
			"Service": {
				"Service": "example-service-via-stdin",
				"OnlyPassing": true
			},
		  	"DNS": {
				"TTL": "60s"
		  	}
         }`))
			stdinW.Close()
		}()

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-",
		}

		code := c.Run(args)
		output := ui.OutputWriter.String()
		stderr := ui.ErrorWriter.String()
		require.Empty(t, stderr)
		require.Contains(t, output, `Query Created: `)
		ID := ""
		fmt.Sscanf(output, "Query Created: %s", &ID)
		require.Equal(t, 0, code)

		queries, _, err := client.PreparedQuery().Get(ID, nil)
		require.NoError(t, err)
		require.Equal(t, 1, len(queries))
		require.Equal(t, "example-query-via-stdin", queries[0].Name)
		require.Equal(t, "example-service-via-stdin", queries[0].Service.Service)

		stdinR, stdinW = io.Pipe()
		c.testStdin = stdinR

		go func() {
			stdinW.Write([]byte(`{
			"ID": "` + ID + `",
			"Name": "example-query-via-stdin",
			"Service": {
				"Service": "example-service-via-stdin2",
				"OnlyPassing": true
			},
		  	"DNS": {
				"TTL": "60s"
		  	}
         }`))
			stdinW.Close()
		}()

		args = []string{
			"-http-addr=" + a.HTTPAddr(),
			"-",
		}

		code = c.Run(args)
		output = ui.OutputWriter.String()
		stderr = ui.ErrorWriter.String()
		require.Empty(t, stderr)
		require.Contains(t, output, `Query Updated: `)
		require.Equal(t, 0, code)

		queries, _, err = client.PreparedQuery().Get(ID, nil)
		require.NoError(t, err)
		require.Equal(t, 1, len(queries))
		require.Equal(t, "example-query-via-stdin", queries[0].Name)
		require.Equal(t, "example-service-via-stdin2", queries[0].Service.Service)
	})
}

func TestQueryWrite_InvalidArgs(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		args []string
		err  string
	}{
		"no id": {
			args: []string{},
			err:  "Must provide exactly one positional argument to specify the prepared query to write",
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			ui := cli.NewMockUi()
			c := New(ui)

			require.NotEqual(t, 0, c.Run(tcase.args))
			require.Contains(t, ui.ErrorWriter.String(), tcase.err)
		})
	}
}
