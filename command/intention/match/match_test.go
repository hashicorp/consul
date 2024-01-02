// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package match

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestIntentionMatch_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(nil).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestIntentionMatch_Validation(t *testing.T) {
	t.Parallel()

	ui := cli.NewMockUi()
	c := New(ui)

	cases := map[string]struct {
		args   []string
		output string
	}{
		"0 args": {
			[]string{},
			"requires exactly one",
		},

		"3 args": {
			[]string{"a", "b", "c"},
			"requires exactly one",
		},

		"both source and dest": {
			[]string{"-source", "-destination", "foo"},
			"only one of -source",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			c.init()

			// Ensure our buffer is always clear
			if ui.ErrorWriter != nil {
				ui.ErrorWriter.Reset()
			}
			if ui.OutputWriter != nil {
				ui.OutputWriter.Reset()
			}

			require.Equal(t, 1, c.Run(tc.args))
			output := ui.ErrorWriter.String()
			require.Contains(t, output, tc.output)
		})
	}
}

func TestIntentionMatch_matchDst(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Create some intentions
	{
		insert := [][]string{
			{"foo", "db"},
			{"web", "db"},
			{"*", "db"},
		}

		for _, v := range insert {
			//nolint:staticcheck
			id, _, err := client.Connect().IntentionCreate(&api.Intention{
				SourceName:      v[0],
				DestinationName: v[1],
				Action:          api.IntentionActionDeny,
			}, nil)
			require.NoError(t, err)
			require.NotEmpty(t, id)
		}
	}

	// Match it
	{
		ui := cli.NewMockUi()
		c := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"db",
		}
		require.Equal(t, 0, c.Run(args), ui.ErrorWriter.String())
		require.Contains(t, ui.OutputWriter.String(), "web")
		require.Contains(t, ui.OutputWriter.String(), "db")
		require.Contains(t, ui.OutputWriter.String(), "*")
	}
}

func TestIntentionMatch_matchSource(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Create some intentions
	{
		insert := [][]string{
			{"foo", "db"},
			{"web", "db"},
			{"*", "db"},
		}

		for _, v := range insert {
			//nolint:staticcheck
			id, _, err := client.Connect().IntentionCreate(&api.Intention{
				SourceName:      v[0],
				DestinationName: v[1],
				Action:          api.IntentionActionDeny,
			}, nil)
			require.NoError(t, err)
			require.NotEmpty(t, id)
		}
	}

	// Match it
	{
		ui := cli.NewMockUi()
		c := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-source",
			"foo",
		}
		require.Equal(t, 0, c.Run(args), ui.ErrorWriter.String())
		require.Contains(t, ui.OutputWriter.String(), "db")
		require.NotContains(t, ui.OutputWriter.String(), "web")
	}
}
