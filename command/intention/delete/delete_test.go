package delete

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestIntentionDelete_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(nil).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestIntentionDelete_Validation(t *testing.T) {
	t.Parallel()

	ui := cli.NewMockUi()
	c := New(ui)

	cases := map[string]struct {
		args   []string
		output string
	}{
		"0 args": {
			[]string{},
			"requires exactly 1 or 2",
		},

		"3 args": {
			[]string{"a", "b", "c"},
			"requires exactly 1 or 2",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c.init()

			// Ensure our buffer is always clear
			if ui.ErrorWriter != nil {
				ui.ErrorWriter.Reset()
			}
			if ui.OutputWriter != nil {
				ui.OutputWriter.Reset()
			}

			require.Equal(t, 1, c.Run(tc.args))
			output := ui.ErrorWriter.String()
			require.Contains(t, output, tc.output)
		})
	}
}

func TestIntentionDelete(t *testing.T) {
	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Create some intentions.

	//nolint:staticcheck
	id0, _, err := client.Connect().IntentionCreate(&api.Intention{
		SourceName:      "web",
		DestinationName: "db",
		Action:          api.IntentionActionDeny,
	}, nil)
	require.NoError(t, err)

	//nolint:staticcheck
	_, _, err = client.Connect().IntentionCreate(&api.Intention{
		SourceName:      "web",
		DestinationName: "queue",
		Action:          api.IntentionActionDeny,
	}, nil)
	require.NoError(t, err)

	t.Run("l4 intention", func(t *testing.T) {
		t.Run("one arg", func(t *testing.T) {
			ui := cli.NewMockUi()
			c := New(ui)

			args := []string{
				"-http-addr=" + a.HTTPAddr(),
				id0,
			}
			require.Equal(t, 0, c.Run(args), ui.ErrorWriter.String())
			require.Contains(t, ui.OutputWriter.String(), "deleted")
		})
		t.Run("two args", func(t *testing.T) {
			ui := cli.NewMockUi()
			c := New(ui)

			args := []string{
				"-http-addr=" + a.HTTPAddr(),
				"web", "queue",
			}
			require.Equal(t, 0, c.Run(args), ui.ErrorWriter.String())
			require.Contains(t, ui.OutputWriter.String(), "deleted")
		})
	})

	// They should all be gone.
	{
		ixns, _, err := client.Connect().Intentions(nil)
		require.NoError(t, err)
		require.Len(t, ixns, 0)
	}
}
