// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package list

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestQueryList(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	_, _, err := client.PreparedQuery().Create(&api.PreparedQueryDefinition{
		Name:    "web",
		Service: api.ServiceQuery{Service: "web"},
	}, nil)
	require.NoError(t, err)

	_, _, err = client.PreparedQuery().Create(&api.PreparedQueryDefinition{
		Name:    "foo",
		Service: api.ServiceQuery{Service: "foo"},
	}, nil)
	require.NoError(t, err)

	_, _, err = client.PreparedQuery().Create(&api.PreparedQueryDefinition{
		Name:    "api",
		Service: api.ServiceQuery{Service: "api"},
	}, nil)
	require.NoError(t, err)

	cases := map[string]struct {
		args     []string
		expected []string
		errMsg   string
	}{
		"list": {
			args: []string{
				"-http-addr=" + a.HTTPAddr(),
			},
			expected: []string{"web", "foo", "api"},
		},
	}
	for name, c := range cases {
		c := c
		t.Run(name, func(t *testing.T) {
			ui := cli.NewMockUi()
			cmd := New(ui)

			code := cmd.Run(c.args)

			if c.errMsg == "" {
				require.Equal(t, 0, code)
				queries := strings.Split(strings.Trim(ui.OutputWriter.String(), "\n"), "\n")
				if len(queries) > 0 {
					queries = queries[1:]
				}
				for i, s := range queries {
					parts := strings.Fields(s)
					queries[i] = parts[0]
				}
				require.ElementsMatch(t, c.expected, queries)
			} else {
				require.NotEqual(t, 0, code)
				require.Contains(t, ui.ErrorWriter.String(), c.errMsg)
			}

		})
	}
}
