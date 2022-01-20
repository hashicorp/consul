package check

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestIntentionCheck_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(nil).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestIntentionCheck_Validation(t *testing.T) {
	t.Parallel()

	ui := cli.NewMockUi()
	c := New(ui)

	cases := map[string]struct {
		args   []string
		output string
	}{
		"0 args": {
			[]string{},
			"requires exactly two",
		},

		"1 args": {
			[]string{"a"},
			"requires exactly two",
		},

		"3 args": {
			[]string{"a", "b", "c"},
			"requires exactly two",
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

			require.Equal(t, 2, c.Run(tc.args))
			output := ui.ErrorWriter.String()
			require.Contains(t, output, tc.output)
		})
	}
}

func TestIntentionCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Create the intention
	{
		//nolint:staticcheck
		_, _, err := client.Connect().IntentionCreate(&api.Intention{
			SourceName:      "web",
			DestinationName: "db",
			Action:          api.IntentionActionDeny,
		}, nil)
		require.NoError(t, err)
	}

	// Get it
	{
		ui := cli.NewMockUi()
		c := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"foo", "db",
		}
		require.Equal(t, 0, c.Run(args), ui.ErrorWriter.String())
		require.Contains(t, ui.OutputWriter.String(), "Allow")
	}

	{
		ui := cli.NewMockUi()
		c := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"web", "db",
		}
		require.Equal(t, 1, c.Run(args), ui.ErrorWriter.String())
		require.Contains(t, ui.OutputWriter.String(), "Denied")
	}
}
