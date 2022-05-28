package read

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestSessionReadCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestSessionReadCommand(t *testing.T) {
	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	c := New(ui)

	id, _, err := client.Session().CreateNoChecks(
		&api.SessionEntry{
			Name: "hello-world",
		},
		nil,
	)
	require.NoError(t, err)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		id,
	}
	require.Equal(t, 0, c.Run(args), ui.ErrorWriter.String())

	output := ui.OutputWriter.String()
	require.Contains(t, output, "hello-world")
	require.Contains(t, output, id)
}

func TestSessionReadCommand_JSON(t *testing.T) {
	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	c := New(ui)

	id, _, err := client.Session().CreateNoChecks(
		&api.SessionEntry{
			Name: "hello-world",
		},
		nil,
	)
	require.NoError(t, err)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-format=json",
		id,
	}
	require.Equal(t, 0, c.Run(args), ui.ErrorWriter.String())

	output := ui.OutputWriter.String()
	require.Contains(t, output, "hello-world")
	require.Contains(t, output, id)

	var jsonOuput json.RawMessage
	err = json.Unmarshal([]byte(output), &jsonOuput)
	require.NoError(t, err)
}

func TestSessionReadCommand_notFound(t *testing.T) {
	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	c := New(ui)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-format=json",
		"1234",
	}
	require.Equal(t, 1, c.Run(args), ui.ErrorWriter.String())
	require.Contains(t, ui.OutputWriter.String(), "")
	require.Contains(t, ui.ErrorWriter.String(), `No session "1234" found`)
}
