package create

import (
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testutil"
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
		"-allow and -deny": {
			[]string{"-allow", "-deny", "foo", "bar"},
			"one of -allow",
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

func TestCommand(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()
	client := a.Client()

	ui := cli.NewMockUi()
	c := New(ui)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"foo", "bar",
	}
	require.Equal(0, c.Run(args), ui.ErrorWriter.String())

	ixns, _, err := client.Connect().Intentions(nil)
	require.NoError(err)
	require.Len(ixns, 1)
	require.Equal("foo", ixns[0].SourceName)
	require.Equal("bar", ixns[0].DestinationName)
	require.Equal(api.IntentionActionAllow, ixns[0].Action)
}

func TestCommand_deny(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()
	client := a.Client()

	ui := cli.NewMockUi()
	c := New(ui)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-deny",
		"foo", "bar",
	}
	require.Equal(0, c.Run(args), ui.ErrorWriter.String())

	ixns, _, err := client.Connect().Intentions(nil)
	require.NoError(err)
	require.Len(ixns, 1)
	require.Equal("foo", ixns[0].SourceName)
	require.Equal("bar", ixns[0].DestinationName)
	require.Equal(api.IntentionActionDeny, ixns[0].Action)
}

func TestCommand_meta(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()
	client := a.Client()

	ui := cli.NewMockUi()
	c := New(ui)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-meta", "hello=world",
		"foo", "bar",
	}
	require.Equal(0, c.Run(args), ui.ErrorWriter.String())

	ixns, _, err := client.Connect().Intentions(nil)
	require.NoError(err)
	require.Len(ixns, 1)
	require.Equal("foo", ixns[0].SourceName)
	require.Equal("bar", ixns[0].DestinationName)
	require.Equal(map[string]string{"hello": "world"}, ixns[0].Meta)
}

func TestCommand_File(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()
	client := a.Client()

	ui := cli.NewMockUi()
	c := New(ui)

	contents := `{ "SourceName": "foo", "DestinationName": "bar", "Action": "allow" }`
	f := testutil.TempFile(t, "intention-create-command-file")
	defer os.Remove(f.Name())
	if _, err := f.WriteString(contents); err != nil {
		t.Fatalf("err: %#v", err)
	}

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-file",
		f.Name(),
	}

	require.Equal(0, c.Run(args), ui.ErrorWriter.String())

	ixns, _, err := client.Connect().Intentions(nil)
	require.NoError(err)
	require.Len(ixns, 1)
	require.Equal("foo", ixns[0].SourceName)
	require.Equal("bar", ixns[0].DestinationName)
	require.Equal(api.IntentionActionAllow, ixns[0].Action)
}

func TestCommand_FileNoExist(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()

	ui := cli.NewMockUi()
	c := New(ui)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-file",
		"shouldnotexist.txt",
	}

	require.Equal(1, c.Run(args), ui.ErrorWriter.String())
	require.Contains(ui.ErrorWriter.String(), "no such file")
}

func TestCommand_replace(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()
	client := a.Client()

	// Create the first
	{
		ui := cli.NewMockUi()
		c := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"foo", "bar",
		}
		require.Equal(0, c.Run(args), ui.ErrorWriter.String())

		ixns, _, err := client.Connect().Intentions(nil)
		require.NoError(err)
		require.Len(ixns, 1)
		require.Equal("foo", ixns[0].SourceName)
		require.Equal("bar", ixns[0].DestinationName)
		require.Equal(api.IntentionActionAllow, ixns[0].Action)
	}

	// Don't replace, should be an error
	{
		ui := cli.NewMockUi()
		c := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-deny",
			"foo", "bar",
		}
		require.Equal(1, c.Run(args), ui.ErrorWriter.String())
		require.Contains(ui.ErrorWriter.String(), "duplicate")
	}

	// Replace it
	{
		ui := cli.NewMockUi()
		c := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-replace",
			"-deny",
			"foo", "bar",
		}
		require.Equal(0, c.Run(args), ui.ErrorWriter.String())

		ixns, _, err := client.Connect().Intentions(nil)
		require.NoError(err)
		require.Len(ixns, 1)
		require.Equal("foo", ixns[0].SourceName)
		require.Equal("bar", ixns[0].DestinationName)
		require.Equal(api.IntentionActionDeny, ixns[0].Action)
	}
}
