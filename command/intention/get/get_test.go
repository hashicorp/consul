package get

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

// TODO(intentions): add test for viewing permissions and ID-less

func TestIntentionGet_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(nil).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestIntentionGet_Validation(t *testing.T) {
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

func TestIntentionGet_id(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

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
		require.NoError(t, err)
	}

	// Get it
	ui := cli.NewMockUi()
	c := New(ui)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		id,
	}
	require.Equal(t, 0, c.Run(args), ui.ErrorWriter.String())
	require.Contains(t, ui.OutputWriter.String(), id)
}

func TestIntentionGet_srcDst(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

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
		require.NoError(t, err)
	}

	// Get it
	ui := cli.NewMockUi()
	c := New(ui)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"web", "db",
	}
	require.Equal(t, 0, c.Run(args), ui.ErrorWriter.String())
	require.Contains(t, ui.OutputWriter.String(), id)
}

func TestIntentionGet_verticalBar(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	sourceName := "source|name|with|bars"

	// Create the intention
	var id string
	{
		var err error
		//nolint:staticcheck
		id, _, err = client.Connect().IntentionCreate(&api.Intention{
			SourceName:      sourceName,
			DestinationName: "db",
			Action:          api.IntentionActionAllow,
		}, nil)
		require.NoError(t, err)
	}

	// Get it
	ui := cli.NewMockUi()
	c := New(ui)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		id,
	}
	require.Equal(t, 0, c.Run(args), ui.ErrorWriter.String())

	// Check for sourceName presense because it should not be parsed by
	// columnize
	require.Contains(t, ui.OutputWriter.String(), sourceName)
}
