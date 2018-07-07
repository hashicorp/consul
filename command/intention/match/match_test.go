package match

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
			"requires exactly one",
		},

		"3 args": {
			[]string{"a", "b", "c"},
			"requires exactly one",
		},

		"both source and dest": {
			[]string{"-source", "-destination", "foo"},
			"only one of -source",
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

			require.Equal(1, c.Run(tc.args))
			output := ui.ErrorWriter.String()
			require.Contains(output, tc.output)
		})
	}
}

func TestCommand_matchDst(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()
	client := a.Client()

	// Create some intentions
	{
		insert := [][]string{
			{"foo", "db"},
			{"web", "db"},
			{"*", "db"},
		}

		for _, v := range insert {
			id, _, err := client.Connect().IntentionCreate(&api.Intention{
				SourceName:      v[0],
				DestinationName: v[1],
				Action:          api.IntentionActionDeny,
			}, nil)
			require.NoError(err)
			require.NotEmpty(id)
		}
	}

	// Match it
	{
		ui := cli.NewMockUi()
		c := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"db",
		}
		require.Equal(0, c.Run(args), ui.ErrorWriter.String())
		require.Contains(ui.OutputWriter.String(), "web")
		require.Contains(ui.OutputWriter.String(), "db")
		require.Contains(ui.OutputWriter.String(), "*")
	}
}

func TestCommand_matchSource(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()
	client := a.Client()

	// Create some intentions
	{
		insert := [][]string{
			{"foo", "db"},
			{"web", "db"},
			{"*", "db"},
		}

		for _, v := range insert {
			id, _, err := client.Connect().IntentionCreate(&api.Intention{
				SourceName:      v[0],
				DestinationName: v[1],
				Action:          api.IntentionActionDeny,
			}, nil)
			require.NoError(err)
			require.NotEmpty(id)
		}
	}

	// Match it
	{
		ui := cli.NewMockUi()
		c := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-source",
			"foo",
		}
		require.Equal(0, c.Run(args), ui.ErrorWriter.String())
		require.Contains(ui.OutputWriter.String(), "db")
		require.NotContains(ui.OutputWriter.String(), "web")
	}
}
