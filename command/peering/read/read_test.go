package read

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
)

func TestReadCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestReadCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	acceptor := agent.NewTestAgent(t, ``)
	t.Cleanup(func() { _ = acceptor.Shutdown() })

	testrpc.WaitForTestAgent(t, acceptor.RPC, "dc1")

	acceptingClient := acceptor.Client()

	t.Run("no name flag", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + acceptor.HTTPAddr(),
		}

		code := cmd.Run(args)
		require.Equal(t, 1, code, "err: %s", ui.ErrorWriter.String())
		require.Contains(t, ui.ErrorWriter.String(), "Missing the required -name flag")
	})

	t.Run("invalid format", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + acceptor.HTTPAddr(),
			"-name=foo",
			"-format=toml",
		}

		code := cmd.Run(args)
		require.Equal(t, 1, code, "exited successfully when it should have failed")
		output := ui.ErrorWriter.String()
		require.Contains(t, output, "Invalid format")
	})

	t.Run("peering does not exist", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + acceptor.HTTPAddr(),
			"-name=foo",
		}

		code := cmd.Run(args)
		require.Equal(t, 1, code, "err: %s", ui.ErrorWriter.String())
		require.Contains(t, ui.ErrorWriter.String(), "No peering with name")
	})

	t.Run("read with pretty print", func(t *testing.T) {

		generateReq := api.PeeringGenerateTokenRequest{
			PeerName: "foo",
			Meta: map[string]string{
				"env": "production",
			},
		}
		_, _, err := acceptingClient.Peerings().GenerateToken(context.Background(), generateReq, &api.WriteOptions{})
		require.NoError(t, err, "Could not generate peering token at acceptor for \"foo\"")

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + acceptor.HTTPAddr(),
			"-name=foo",
		}

		code := cmd.Run(args)
		require.Equal(t, 0, code)
		output := ui.OutputWriter.String()
		require.Greater(t, strings.Count(output, "\n"), 0) // Checking for some kind of empty output

		// Spot check some fields and values
		require.Contains(t, output, "foo")
		require.Contains(t, output, api.PeeringStatePending)
		require.Contains(t, output, "env=production")
		require.Contains(t, output, "Imported Services")
		require.Contains(t, output, "Exported Services")
	})

	t.Run("read with json", func(t *testing.T) {

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + acceptor.HTTPAddr(),
			"-name=foo",
			"-format=json",
		}

		code := cmd.Run(args)
		require.Equal(t, 0, code)
		output := ui.OutputWriter.Bytes()

		var outputPeering api.Peering
		require.NoError(t, json.Unmarshal(output, &outputPeering))

		require.Equal(t, "foo", outputPeering.Name)
		require.Equal(t, "production", outputPeering.Meta["env"])
	})
}
