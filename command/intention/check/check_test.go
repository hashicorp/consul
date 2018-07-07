package check

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(nil).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestCommand_Validation(t *testing.T) {
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
			require := require.New(t)

			c.init()

			// Ensure our buffer is always clear
			if ui.ErrorWriter != nil {
				ui.ErrorWriter.Reset()
			}
			if ui.OutputWriter != nil {
				ui.OutputWriter.Reset()
			}

			require.Equal(2, c.Run(tc.args))
			output := ui.ErrorWriter.String()
			require.Contains(output, tc.output)
		})
	}
}

func TestCommand(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()
	client := a.Client()

	// Create the intention
	{
		_, _, err := client.Connect().IntentionCreate(&api.Intention{
			SourceName:      "web",
			DestinationName: "db",
			Action:          api.IntentionActionDeny,
		}, nil)
		require.NoError(err)
	}

	// Get it
	{
		ui := cli.NewMockUi()
		c := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"foo", "db",
		}
		require.Equal(0, c.Run(args), ui.ErrorWriter.String())
		require.Contains(ui.OutputWriter.String(), "Allow")
	}

	{
		ui := cli.NewMockUi()
		c := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"web", "db",
		}
		require.Equal(1, c.Run(args), ui.ErrorWriter.String())
		require.Contains(ui.OutputWriter.String(), "Denied")
	}
}
