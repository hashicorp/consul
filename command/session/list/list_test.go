package list

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestSessionListCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestSessionListCommand(t *testing.T) {
	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	testrpc.WaitForTestAgent(t, a.RPC, ``)

	ui := cli.NewMockUi()
	c := New(ui)

	var ids []string
	for i := 0; i < 5; i++ {
		id, _, err := client.Session().CreateNoChecks(
			&api.SessionEntry{
				Name: fmt.Sprintf("hello-world-%d", i),
			},
			nil,
		)
		require.NoError(t, err)
		ids = append(ids, id)
	}

	node, err := client.Agent().NodeName()
	require.NoError(t, err)

	cases := map[string]struct {
		args           []string
		expectSessions bool
	}{
		"global": {
			args: []string{
				"-http-addr=" + a.HTTPAddr(),
			},
			expectSessions: true,
		},
		"node": {
			args: []string{
				"-http-addr=" + a.HTTPAddr(),
				"-node=" + node,
			},
			expectSessions: true,
		},
		"unknown-node": {
			args: []string{
				"-http-addr=" + a.HTTPAddr(),
				"-node=1234",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			retry.Run(t, func(r *retry.R) {

				ui.OutputWriter.Reset()
				ui.ErrorWriter.Reset()

				require.Equal(r, 0, c.Run(tc.args), ui.ErrorWriter.String())

				output := ui.OutputWriter.String()

				if tc.expectSessions {
					for i, v := range ids {
						require.Contains(r, output, fmt.Sprintf("hello-world-%d", i))
						require.Contains(r, output, v)
					}
				}
			})
		})
	}
}

func TestSessionListCommand_JSON(t *testing.T) {
	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	c := New(ui)

	var ids []string
	for i := 0; i < 5; i++ {
		id, _, err := client.Session().CreateNoChecks(
			&api.SessionEntry{
				Name: fmt.Sprintf("hello-world-%d", i),
			},
			nil,
		)
		require.NoError(t, err)
		ids = append(ids, id)
	}

	node, err := client.Agent().NodeName()
	require.NoError(t, err)

	cases := map[string]struct {
		args           []string
		expectSessions bool
	}{
		"global": {
			args: []string{
				"-http-addr=" + a.HTTPAddr(),
				"-format=json",
			},
			expectSessions: true,
		},
		"node": {
			args: []string{
				"-http-addr=" + a.HTTPAddr(),
				"-format=json",
				"-node=" + node,
			},
			expectSessions: true,
		},
		"unknown-node": {
			args: []string{
				"-http-addr=" + a.HTTPAddr(),
				"-format=json",
				"-node=1234",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			retry.Run(t, func(r *retry.R) {
				ui.OutputWriter.Reset()
				ui.ErrorWriter.Reset()

				require.Equal(r, 0, c.Run(tc.args), ui.ErrorWriter.String())

				output := ui.OutputWriter.String()

				if tc.expectSessions {
					for i, v := range ids {
						require.Contains(r, output, fmt.Sprintf("hello-world-%d", i))
						require.Contains(r, output, v)
					}
				}

				var jsonOutput json.RawMessage
				err := json.Unmarshal([]byte(output), &jsonOutput)
				require.NoError(r, err, output)
			})
		})
	}
}
