package get

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
			"requires exactly 1 or 2",
		},

		"3 args": {
			[]string{"a", "b", "c"},
			"requires exactly 1 or 2",
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

func TestCommand_id(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()
	client := a.Client()

	// Create the intention
	var id string
	{
		var err error
		id, _, err = client.Connect().IntentionCreate(&api.Intention{
			SourceName:      "web",
			DestinationName: "db",
			Action:          api.IntentionActionAllow,
		}, nil)
		require.NoError(err)
	}

	// Get it
	ui := cli.NewMockUi()
	c := New(ui)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		id,
	}
	require.Equal(0, c.Run(args), ui.ErrorWriter.String())
	require.Contains(ui.OutputWriter.String(), id)
}

func TestCommand_srcDst(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()
	client := a.Client()

	// Create the intention
	var id string
	{
		var err error
		id, _, err = client.Connect().IntentionCreate(&api.Intention{
			SourceName:      "web",
			DestinationName: "db",
			Action:          api.IntentionActionAllow,
		}, nil)
		require.NoError(err)
	}

	// Get it
	ui := cli.NewMockUi()
	c := New(ui)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"web", "db",
	}
	require.Equal(0, c.Run(args), ui.ErrorWriter.String())
	require.Contains(ui.OutputWriter.String(), id)
}
