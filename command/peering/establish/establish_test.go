package establish

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
)

func TestEstablishCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestEstablishCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	acceptor := agent.NewTestAgent(t, ``)
	t.Cleanup(func() { _ = acceptor.Shutdown() })

	dialer := agent.NewTestAgent(t, `datacenter = "dc2"`)
	t.Cleanup(func() { _ = dialer.Shutdown() })

	testrpc.WaitForTestAgent(t, acceptor.RPC, "dc1")
	testrpc.WaitForTestAgent(t, dialer.RPC, "dc2")

	acceptingClient := acceptor.Client()
	dialingClient := dialer.Client()

	t.Run("name is required", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + dialer.HTTPAddr(),
			"-peering-token=1234abcde",
		}

		code := cmd.Run(args)
		require.Equal(t, 1, code, "err: %s", ui.ErrorWriter.String())
		require.Contains(t, ui.ErrorWriter.String(), "Missing the required -name flag")
	})

	t.Run("peering token is required", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + dialer.HTTPAddr(),
			"-name=bar",
		}

		code := cmd.Run(args)
		require.Equal(t, 1, code, "err: %s", ui.ErrorWriter.String())
		require.Contains(t, ui.ErrorWriter.String(), "Missing the required -peering-token flag")
	})

	t.Run("establish connection", func(t *testing.T) {
		// Grab the token from the acceptor
		req := api.PeeringGenerateTokenRequest{PeerName: "foo"}
		res, _, err := acceptingClient.Peerings().GenerateToken(context.Background(), req, &api.WriteOptions{})
		require.NoError(t, err, "Could not generate peering token at acceptor")

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + dialer.HTTPAddr(),
			"-name=bar",
			fmt.Sprintf("-peering-token=%s", res.PeeringToken),
		}

		retry.Run(t, func(r *retry.R) {
			code := cmd.Run(args)
			require.Equal(r, 0, code)
			output := ui.OutputWriter.String()
			require.Contains(r, output, "Success")
		})
	})

	t.Run("establish connection with options", func(t *testing.T) {
		// Grab the token from the acceptor
		req := api.PeeringGenerateTokenRequest{PeerName: "foo"}
		res, _, err := acceptingClient.Peerings().GenerateToken(context.Background(), req, &api.WriteOptions{})
		require.NoError(t, err, "Could not generate peering token at acceptor")

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + dialer.HTTPAddr(),
			"-name=bar",
			fmt.Sprintf("-peering-token=%s", res.PeeringToken),
			"-meta=env=production",
			"-meta=region=us-west-1",
		}

		retry.Run(t, func(r *retry.R) {
			code := cmd.Run(args)
			require.Equal(r, 0, code)
			output := ui.OutputWriter.String()
			require.Contains(r, output, "Success")
		})

		// Meta
		peering, _, err := dialingClient.Peerings().Read(context.Background(), "bar", &api.QueryOptions{})
		require.NoError(t, err)

		actual, ok := peering.Meta["env"]
		require.True(t, ok)
		require.Equal(t, "production", actual)

		actual, ok = peering.Meta["region"]
		require.True(t, ok)
		require.Equal(t, "us-west-1", actual)
	})
}
