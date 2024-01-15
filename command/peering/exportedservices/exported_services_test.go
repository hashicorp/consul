// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package exportedservices

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
)

func TestExportedServicesCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestExportedServicesCommand(t *testing.T) {
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

	t.Run("peering exist but no exported services", func(t *testing.T) {
		// Generate token
		generateReq := api.PeeringGenerateTokenRequest{
			PeerName: "foo",
			Meta: map[string]string{
				"env": "production",
			},
		}

		res, _, err := acceptingClient.Peerings().GenerateToken(context.Background(), generateReq, &api.WriteOptions{})
		require.NoError(t, err, "Could not generate peering token at acceptor for \"foo\"")

		// Establish peering
		establishReq := api.PeeringEstablishRequest{
			PeerName:     "bar",
			PeeringToken: res.PeeringToken,
			Meta: map[string]string{
				"env": "production",
			},
		}

		_, _, err = dialingClient.Peerings().Establish(context.Background(), establishReq, &api.WriteOptions{})
		require.NoError(t, err, "Could not establish peering for \"bar\"")

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + acceptor.HTTPAddr(),
			"-name=foo",
		}

		code := cmd.Run(args)
		require.Equal(t, 0, code)
		require.Equal(t, ui.ErrorWriter.String(), "")
	})

	t.Run("exported-services with pretty print", func(t *testing.T) {
		// Generate token
		generateReq := api.PeeringGenerateTokenRequest{
			PeerName: "foo",
			Meta: map[string]string{
				"env": "production",
			},
		}

		res, _, err := acceptingClient.Peerings().GenerateToken(context.Background(), generateReq, &api.WriteOptions{})
		require.NoError(t, err, "Could not generate peering token at acceptor for \"foo\"")

		// Establish peering
		establishReq := api.PeeringEstablishRequest{
			PeerName:     "bar",
			PeeringToken: res.PeeringToken,
			Meta: map[string]string{
				"env": "production",
			},
		}

		_, _, err = dialingClient.Peerings().Establish(context.Background(), establishReq, &api.WriteOptions{})
		require.NoError(t, err, "Could not establish peering for \"bar\"")

		_, _, err = acceptingClient.ConfigEntries().Set(&api.ExportedServicesConfigEntry{
			Name: "default",
			Services: []api.ExportedService{
				{
					Name: "web",
					Consumers: []api.ServiceConsumer{
						{
							Peer: "foo",
						},
					},
				},
				{
					Name: "db",
					Consumers: []api.ServiceConsumer{
						{
							Peer: "foo",
						},
					},
				},
			},
		}, nil)
		require.NoError(t, err)

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + acceptor.HTTPAddr(),
			"-name=foo",
		}

		retry.Run(t, func(r *retry.R) {
			code := cmd.Run(args)
			require.Equal(r, 0, code)
			output := ui.OutputWriter.String()

			// Spot check some fields and values
			require.Contains(r, output, "web")
			require.Contains(r, output, "db")
		})
	})

	t.Run("exported-services with json", func(t *testing.T) {

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

		var services []structs.ServiceID
		require.NoError(t, json.Unmarshal(output, &services))

		require.Equal(t, "db", services[0].ID)
		require.Equal(t, "web", services[1].ID)
	})
}
