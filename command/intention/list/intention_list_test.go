package list

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestIntentionListCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestIntentionListCommand(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	// Create the intention
	var id string
	{
		var err error
		//nolint:staticcheck
		id, _, err = client.Connect().IntentionCreate(&api.Intention{
			SourceName:      "web",
			DestinationName: "db",
			Action:          api.IntentionActionAllow,
		}, nil)
		require.NoError(err)
	}

	// List all intentions
	ui := cli.NewMockUi()
	cmd := New(ui)
	args := []string{"-http-addr=" + a.HTTPAddr()}

	require.Equal(0, cmd.Run(args), ui.ErrorWriter.String())
	require.Contains(ui.OutputWriter.String(), id)
}
