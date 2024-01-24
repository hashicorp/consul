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

func TestExportedServices(t *testing.T) {
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
				Name: "web",
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
		},
	}, nil)
	require.NoError(t, err)
	require.True(t, set)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
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
}
