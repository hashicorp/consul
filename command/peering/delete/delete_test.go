// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package delete

import (
	"context"
	"strings"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
)

func TestDeleteCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestDeleteCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	acceptor := agent.NewTestAgent(t, ``)
	t.Cleanup(func() { _ = acceptor.Shutdown() })

	testrpc.WaitForTestAgent(t, acceptor.RPC, "dc1")

	acceptingClient := acceptor.Client()

	t.Run("name is required", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + acceptor.HTTPAddr(),
		}

		code := cmd.Run(args)
		require.Equal(t, 1, code, "err: %s", ui.ErrorWriter.String())
		require.Contains(t, ui.ErrorWriter.String(), "Missing the required -name flag")
	})

	t.Run("delete connection", func(t *testing.T) {

		req := api.PeeringGenerateTokenRequest{PeerName: "foo"}
		_, _, err := acceptingClient.Peerings().GenerateToken(context.Background(), req, &api.WriteOptions{})
		require.NoError(t, err, "Could not generate peering token at acceptor")

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + acceptor.HTTPAddr(),
			"-name=foo",
		}

		code := cmd.Run(args)
		require.Equal(t, 0, code)
		output := ui.OutputWriter.String()
		require.Contains(t, output, "Success")
	})
}
