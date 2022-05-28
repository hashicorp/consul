package create

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestSessionCreate_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(nil).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestSessionCreate(t *testing.T) {
	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	c := New(ui)

	retry.Run(t, func(r *retry.R) {
		ui.ErrorWriter.Reset()
		ui.OutputWriter.Reset()

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-node-check=serfHealth",
		}
		require.Equal(r, 0, c.Run(args), ui.ErrorWriter.String())

		s, _, err := client.Session().List(nil)
		require.NoError(r, err)
		require.Len(r, s, 1)
		require.Equal(r, s[0].ID, strings.TrimSpace(ui.OutputWriter.String()))
	})
}

func TestSessionCreate_Namespace(t *testing.T) {
	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	c := New(ui)

	node, err := client.Agent().NodeName()
	require.NoError(t, err)

	_, err = client.Catalog().Register(
		&api.CatalogRegistration{
			Node:           node,
			SkipNodeUpdate: true,
			Check: &api.AgentCheck{
				Name:   "hello",
				Type:   "ttl",
				Status: "pass",
			},
		},
		nil,
	)
	require.NoError(t, err)

	retry.Run(t, func(r *retry.R) {
		ui.ErrorWriter.Reset()
		ui.OutputWriter.Reset()

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-service-check=hello:default",
		}
		require.Equal(r, 0, c.Run(args), ui.ErrorWriter.String())

		s, _, err := client.Session().List(nil)
		require.NoError(r, err)
		require.Len(r, s, 1)
		require.Equal(r, s[0].ID, strings.TrimSpace(ui.OutputWriter.String()))
	})
}
