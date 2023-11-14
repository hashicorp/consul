// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package list

import (
	"strings"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

func TestIntentionListCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestIntentionListCommand(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	// Create the intention
	var id string
	{
		var err error
		// This needs to be in a retry in 1.9+ due to the potential to get errors about
		// intentions being read only during intention -> config entry migration.
		retry.Run(t, func(r *retry.R) {
			//nolint:staticcheck
			id, _, err = client.Connect().IntentionCreate(&api.Intention{
				SourceName:      "web",
				DestinationName: "db",
				Action:          api.IntentionActionAllow,
			}, nil)
			require.NoError(r, err)
		})
	}

	// List all intentions
	ui := cli.NewMockUi()
	cmd := New(ui)
	args := []string{"-http-addr=" + a.HTTPAddr()}

	require.Equal(t, 0, cmd.Run(args), ui.ErrorWriter.String())
	require.Contains(t, ui.OutputWriter.String(), id)
}
