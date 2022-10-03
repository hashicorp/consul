package list

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

func TestListCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestListCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	acceptor := agent.NewTestAgent(t, ``)
	t.Cleanup(func() { _ = acceptor.Shutdown() })

	testrpc.WaitForTestAgent(t, acceptor.RPC, "dc1")

	acceptingClient := acceptor.Client()

	t.Run("invalid format", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + acceptor.HTTPAddr(),
			"-format=toml",
		}

		code := cmd.Run(args)
		require.Equal(t, 1, code, "exited successfully when it should have failed")
		output := ui.ErrorWriter.String()
		require.Contains(t, output, "Invalid format")
	})

	t.Run("no results - pretty", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + acceptor.HTTPAddr(),
		}

		code := cmd.Run(args)
		require.Equal(t, 0, code)
		output := ui.OutputWriter.String()
		require.Contains(t, output, "no peering connections")
	})

	t.Run("no results - json", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + acceptor.HTTPAddr(),
			"-format=json",
		}

		code := cmd.Run(args)
		require.Equal(t, 0, code)
		output := ui.OutputWriter.String()
		require.Contains(t, output, "[]")
	})

	t.Run("two results for pretty print", func(t *testing.T) {

		generateReq := api.PeeringGenerateTokenRequest{PeerName: "foo"}
		_, _, err := acceptingClient.Peerings().GenerateToken(context.Background(), generateReq, &api.WriteOptions{})
		require.NoError(t, err, "Could not generate peering token at acceptor for \"foo\"")

		generateReq = api.PeeringGenerateTokenRequest{PeerName: "bar"}
		_, _, err = acceptingClient.Peerings().GenerateToken(context.Background(), generateReq, &api.WriteOptions{})
		require.NoError(t, err, "Could not generate peering token at acceptor for \"bar\"")

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + acceptor.HTTPAddr(),
		}

		code := cmd.Run(args)
		require.Equal(t, 0, code)
		output := ui.OutputWriter.String()
		require.Equal(t, 3, strings.Count(output, "\n")) // There should be three lines including the header

		lines := strings.Split(output, "\n")

		require.Contains(t, lines[0], "Name")
		require.Contains(t, lines[1], "bar")
		require.Contains(t, lines[2], "foo")
	})

	t.Run("two results for JSON print", func(t *testing.T) {

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + acceptor.HTTPAddr(),
			"-format=json",
		}

		code := cmd.Run(args)
		require.Equal(t, 0, code)
		output := ui.OutputWriter.Bytes()

		var outputList []*api.Peering
		require.NoError(t, json.Unmarshal(output, &outputList))

		require.Len(t, outputList, 2)
		require.Equal(t, "bar", outputList[0].Name)
		require.Equal(t, "foo", outputList[1].Name)
	})
}
