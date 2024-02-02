// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package exportedservices

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/mitchellh/cli"

	"github.com/stretchr/testify/require"
)

func TestExportedServices_noTabs(t *testing.T) {
	t.Parallel()

	require.NotContains(t, New(cli.NewMockUi()).Help(), "\t")
}

func TestExportedServices_Error(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()

	t.Run("No exported services", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
		}

		code := cmd.Run(args)
		require.Equal(t, 0, code)

		output := ui.OutputWriter.String()
		require.Equal(t, "No exported services found\n", output)
	})

	t.Run("invalid format", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-format=toml",
		}

		code := cmd.Run(args)
		require.Equal(t, 1, code, "exited successfully when it should have failed")
		output := ui.ErrorWriter.String()
		require.Contains(t, output, "Invalid format")
	})
}

func TestExportedServices_Pretty(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	ui := cli.NewMockUi()
	c := New(ui)

	set, _, err := client.ConfigEntries().Set(&api.ExportedServicesConfigEntry{
		Name: "default",
		Services: []api.ExportedService{
			{
				Name: "db",
				Consumers: []api.ServiceConsumer{
					{
						Peer: "east",
					},
					{
						Peer: "west",
					},
				},
			},
			{
				Name: "web",
				Consumers: []api.ServiceConsumer{
					{
						Peer: "east",
					},
				},
			},
		},
	}, nil)
	require.NoError(t, err)
	require.True(t, set)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
	}

	code := c.Run(args)
	require.Equal(t, 0, code)

	output := ui.OutputWriter.String()

	// Spot check some fields and values
	require.Contains(t, output, "db")
	require.Contains(t, output, "web")
}

func TestExportedServices_JSON(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	ui := cli.NewMockUi()
	c := New(ui)

	set, _, err := client.ConfigEntries().Set(&api.ExportedServicesConfigEntry{
		Name: "default",
		Services: []api.ExportedService{
			{
				Name: "db",
				Consumers: []api.ServiceConsumer{
					{
						Peer: "east",
					},
					{
						Peer: "west",
					},
				},
			},
			{
				Name: "web",
				Consumers: []api.ServiceConsumer{
					{
						Peer: "east",
					},
				},
			},
		},
	}, nil)
	require.NoError(t, err)
	require.True(t, set)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-format=json",
	}

	code := c.Run(args)
	require.Equal(t, 0, code)

	var resp []api.ResolvedExportedService

	err = json.Unmarshal(ui.OutputWriter.Bytes(), &resp)
	require.NoError(t, err)

	require.Equal(t, 2, len(resp))
	require.Equal(t, "db", resp[0].Service)
	require.Equal(t, "web", resp[1].Service)
	require.Equal(t, []string{"east", "west"}, resp[0].Consumers.Peers)
	require.Equal(t, []string{"east"}, resp[1].Consumers.Peers)
}

func TestExportedServices_filter(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	set, _, err := client.ConfigEntries().Set(&api.ExportedServicesConfigEntry{
		Name: "default",
		Services: []api.ExportedService{
			{
				Name: "db",
				Consumers: []api.ServiceConsumer{
					{
						Peer: "east",
					},
					{
						Peer: "west",
					},
				},
			},
			{
				Name: "web",
				Consumers: []api.ServiceConsumer{
					{
						Peer: "east",
					},
				},
			},
			{
				Name: "backend",
				Consumers: []api.ServiceConsumer{
					{
						Peer: "west",
					},
				},
			},
			{
				Name: "frontend",
				Consumers: []api.ServiceConsumer{
					{
						Peer: "peer1",
					},
					{
						Peer: "peer2",
					},
				},
			},
		},
	}, nil)
	require.NoError(t, err)
	require.True(t, set)

	t.Run("consumerPeer=east", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-format=json",
			"-filter=" + `east in Consumers.Peers`,
		}

		code := cmd.Run(args)
		require.Equal(t, 0, code)

		var resp []api.ResolvedExportedService
		err = json.Unmarshal(ui.OutputWriter.Bytes(), &resp)
		require.NoError(t, err)

		require.Equal(t, 2, len(resp))
		require.Equal(t, "db", resp[0].Service)
		require.Equal(t, "web", resp[1].Service)
		require.Equal(t, []string{"east", "west"}, resp[0].Consumers.Peers)
		require.Equal(t, []string{"east"}, resp[1].Consumers.Peers)

	})

	t.Run("consumerPeer=west", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-format=json",
			"-filter=" + `west in Consumers.Peers`,
		}

		code := cmd.Run(args)
		require.Equal(t, 0, code)

		var resp []api.ResolvedExportedService
		err = json.Unmarshal(ui.OutputWriter.Bytes(), &resp)
		require.NoError(t, err)

		require.Equal(t, 2, len(resp))
		require.Equal(t, "backend", resp[0].Service)
		require.Equal(t, "db", resp[1].Service)
		require.Equal(t, []string{"west"}, resp[0].Consumers.Peers)
		require.Equal(t, []string{"east", "west"}, resp[1].Consumers.Peers)
	})

	t.Run("consumerPeer=peer1", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-format=json",
			"-filter=" + `peer1 in Consumers.Peers`,
		}

		code := cmd.Run(args)
		require.Equal(t, 0, code)

		var resp []api.ResolvedExportedService
		err = json.Unmarshal(ui.OutputWriter.Bytes(), &resp)
		require.NoError(t, err)

		require.Equal(t, 1, len(resp))
		require.Equal(t, "frontend", resp[0].Service)
		require.Equal(t, []string{"peer1", "peer2"}, resp[0].Consumers.Peers)
	})

	t.Run("No exported services", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-filter=" + `unknown in Consumers.Peers`,
		}

		code := cmd.Run(args)
		require.Equal(t, 0, code)

		output := ui.OutputWriter.String()
		require.Equal(t, "No exported services found\n", output)
	})
}
