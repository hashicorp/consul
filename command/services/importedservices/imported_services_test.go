// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package importedservices

import (
	"encoding/json"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
)

func TestImportedServices_noTabs(t *testing.T) {
	t.Parallel()

	require.NotContains(t, New(cli.NewMockUi()).Help(), "\t")
}

func TestImportedServices_Error(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()

	t.Run("No imported services", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
		}

		code := cmd.Run(args)
		require.Equal(t, 0, code)

		output := ui.OutputWriter.String()
		require.Equal(t, "No imported services found\n", output)
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

func TestImportedServices_Pretty(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	ui := cli.NewMockUi()
	c := New(ui)

	set, _, err := client.ConfigEntries().Set(&api.ServiceIntentionsConfigEntry{
		Kind: api.ServiceIntentions,
		Name: "db",
		Sources: []*api.SourceIntention{
			{
				Name:   "web",
				Action: api.IntentionActionAllow,
				Peer:   "east",
			},
		},
	}, nil)
	require.NoError(t, err)
	require.True(t, set)

	set, _, err = client.ConfigEntries().Set(&api.ServiceIntentionsConfigEntry{
		Kind: api.ServiceIntentions,
		Name: "api",
		Sources: []*api.SourceIntention{
			{
				Name:   "frontend",
				Action: api.IntentionActionAllow,
				Peer:   "west",
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
	require.Contains(t, output, "api")
}

func TestImportedServices_JSON(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	ui := cli.NewMockUi()
	c := New(ui)

	set, _, err := client.ConfigEntries().Set(&api.ServiceIntentionsConfigEntry{
		Kind: api.ServiceIntentions,
		Name: "db",
		Sources: []*api.SourceIntention{
			{
				Name:   "web",
				Action: api.IntentionActionAllow,
				Peer:   "east",
			},
		},
	}, nil)
	require.NoError(t, err)
	require.True(t, set)

	set, _, err = client.ConfigEntries().Set(&api.ServiceIntentionsConfigEntry{
		Kind: api.ServiceIntentions,
		Name: "api",
		Sources: []*api.SourceIntention{
			{
				Name:   "frontend",
				Action: api.IntentionActionAllow,
				Peer:   "west",
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

	var resp []api.ResolvedImportedService

	err = json.Unmarshal(ui.OutputWriter.Bytes(), &resp)
	require.NoError(t, err)

	require.Equal(t, 2, len(resp))
	require.Equal(t, "api", resp[0].Service)
	require.Equal(t, "db", resp[1].Service)
	require.Equal(t, "west", resp[0].SourcePeer)
	require.Equal(t, "east", resp[1].SourcePeer)
}

func TestImportedServices_filter(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	set, _, err := client.ConfigEntries().Set(&api.ServiceIntentionsConfigEntry{
		Kind: api.ServiceIntentions,
		Name: "db",
		Sources: []*api.SourceIntention{
			{
				Name:   "web",
				Action: api.IntentionActionAllow,
				Peer:   "east",
			},
		},
	}, nil)
	require.NoError(t, err)
	require.True(t, set)

	set, _, err = client.ConfigEntries().Set(&api.ServiceIntentionsConfigEntry{
		Kind: api.ServiceIntentions,
		Name: "api",
		Sources: []*api.SourceIntention{
			{
				Name:   "frontend",
				Action: api.IntentionActionAllow,
				Peer:   "west",
			},
		},
	}, nil)
	require.NoError(t, err)
	require.True(t, set)

	set, _, err = client.ConfigEntries().Set(&api.ServiceIntentionsConfigEntry{
		Kind: api.ServiceIntentions,
		Name: "backend",
		Sources: []*api.SourceIntention{
			{
				Name:   "service1",
				Action: api.IntentionActionAllow,
				Peer:   "west",
			},
		},
	}, nil)
	require.NoError(t, err)
	require.True(t, set)

	set, _, err = client.ConfigEntries().Set(&api.ServiceIntentionsConfigEntry{
		Kind: api.ServiceIntentions,
		Name: "frontend",
		Sources: []*api.SourceIntention{
			{
				Name:   "service2",
				Action: api.IntentionActionAllow,
				Peer:   "peer1",
			},
		},
	}, nil)
	require.NoError(t, err)
	require.True(t, set)

	t.Run("sourcePeer=east", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-format=json",
			"-filter=" + `SourcePeer == "east"`,
		}

		code := cmd.Run(args)
		require.Equal(t, 0, code)

		var resp []api.ResolvedImportedService
		err = json.Unmarshal(ui.OutputWriter.Bytes(), &resp)
		require.NoError(t, err)

		require.Equal(t, 1, len(resp))
		require.Equal(t, "db", resp[0].Service)
		require.Equal(t, "east", resp[0].SourcePeer)
	})

	t.Run("sourcePeer=west", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-format=json",
			"-filter=" + `SourcePeer == "west"`,
		}

		code := cmd.Run(args)
		require.Equal(t, 0, code)

		var resp []api.ResolvedImportedService
		err = json.Unmarshal(ui.OutputWriter.Bytes(), &resp)
		require.NoError(t, err)

		require.Equal(t, 2, len(resp))
		require.Equal(t, "api", resp[0].Service)
		require.Equal(t, "backend", resp[1].Service)
		require.Equal(t, "west", resp[0].SourcePeer)
		require.Equal(t, "west", resp[1].SourcePeer)
	})

	t.Run("sourcePeer=peer1", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-format=json",
			"-filter=" + `SourcePeer == "peer1"`,
		}

		code := cmd.Run(args)
		require.Equal(t, 0, code)

		var resp []api.ResolvedImportedService
		err = json.Unmarshal(ui.OutputWriter.Bytes(), &resp)
		require.NoError(t, err)

		require.Equal(t, 1, len(resp))
		require.Equal(t, "frontend", resp[0].Service)
		require.Equal(t, "peer1", resp[0].SourcePeer)
	})

	t.Run("No imported services", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-filter=" + `SourcePeer == "unknown"`,
		}

		code := cmd.Run(args)
		require.Equal(t, 0, code)

		output := ui.OutputWriter.String()
		require.Equal(t, "No imported services found\n", output)
	})
}
