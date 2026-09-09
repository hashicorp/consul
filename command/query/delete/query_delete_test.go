// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package delete

import (
	"fmt"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestQueryDelete_noTabs(t *testing.T) {
	t.Parallel()

	require.NotContains(t, New(cli.NewMockUi()).Help(), "\t")
}

func TestQueryDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	ui := cli.NewMockUi()
	c := New(ui)

	ID, _, err := client.PreparedQuery().Create(&api.PreparedQueryDefinition{
		Name:    "web",
		Service: api.ServiceQuery{Service: "web"},
	}, nil)
	require.NoError(t, err)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		fmt.Sprintf("-id=%s", ID),
	}

	code := c.Run(args)
	require.Equal(t, 0, code)

	_, _, err = client.PreparedQuery().Get(ID, nil)
	require.Error(t, err)
}

func TestQueryDelete_InvalidArgs(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		args []string
		err  string
	}{
		"no id": {
			args: []string{},
			err:  "Must specify the -id parameter",
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
