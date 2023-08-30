// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package create

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/testrpc"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestIntentionCreate_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(nil).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestIntentionCreate_Validation(t *testing.T) {
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

func TestIntentionCreate(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	c := New(ui)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"foo", "bar",
	}
	require.Equal(t, 0, c.Run(args), ui.ErrorWriter.String())

	ixns, _, err := client.Connect().Intentions(nil)
	require.NoError(t, err)
	require.Len(t, ixns, 1)
	require.Equal(t, "foo", ixns[0].SourceName)
	require.Equal(t, "bar", ixns[0].DestinationName)
	require.Equal(t, api.IntentionActionAllow, ixns[0].Action)
}

func TestIntentionCreate_deny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	c := New(ui)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-deny",
		"foo", "bar",
	}
	require.Equal(t, 0, c.Run(args), ui.ErrorWriter.String())

	ixns, _, err := client.Connect().Intentions(nil)
	require.NoError(t, err)
	require.Len(t, ixns, 1)
	require.Equal(t, "foo", ixns[0].SourceName)
	require.Equal(t, "bar", ixns[0].DestinationName)
	require.Equal(t, api.IntentionActionDeny, ixns[0].Action)
}

func TestIntentionCreate_meta(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	c := New(ui)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-meta", "hello=world",
		"foo", "bar",
	}
	require.Equal(t, 0, c.Run(args), ui.ErrorWriter.String())

	ixns, _, err := client.Connect().Intentions(nil)
	require.NoError(t, err)
	require.Len(t, ixns, 1)
	require.Equal(t, "foo", ixns[0].SourceName)
	require.Equal(t, "bar", ixns[0].DestinationName)
	require.Equal(t, map[string]string{"hello": "world"}, ixns[0].Meta)
}

func TestIntentionCreate_File(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	c := New(ui)

	contents := `{ "SourceName": "foo", "DestinationName": "bar", "Action": "allow" }`
	f := testutil.TempFile(t, "intention-create-command-file")
	if _, err := f.WriteString(contents); err != nil {
		t.Fatalf("err: %#v", err)
	}

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-file",
		f.Name(),
	}

	require.Equal(t, 0, c.Run(args), ui.ErrorWriter.String())

	ixns, _, err := client.Connect().Intentions(nil)
	require.NoError(t, err)
	require.Len(t, ixns, 1)
	require.Equal(t, "foo", ixns[0].SourceName)
	require.Equal(t, "bar", ixns[0].DestinationName)
	require.Equal(t, api.IntentionActionAllow, ixns[0].Action)
}

func TestIntentionCreate_File_L7_fails(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	c := New(ui)

	contents := `
{
  "SourceName": "foo",
  "DestinationName": "bar",
  "Permissions": [
    {
      "Action": "allow",
      "HTTP": {
        "PathExact": "/foo"
      }
    }
  ]
}
	`
	f := testutil.TempFile(t, "intention-create-command-file")
	if _, err := f.WriteString(contents); err != nil {
		t.Fatalf("err: %#v", err)
	}

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-file",
		f.Name(),
	}

	require.Equal(t, 1, c.Run(args), ui.ErrorWriter.String())
	require.Contains(t, ui.ErrorWriter.String(), "cannot create L7 intention from file")
}

func TestIntentionCreate_FileNoExist(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	c := New(ui)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-file",
		"shouldnotexist.txt",
	}

	require.Equal(t, 1, c.Run(args), ui.ErrorWriter.String())
	require.Contains(t, ui.ErrorWriter.String(), "no such file")
}

func TestIntentionCreate_replace(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Create the first
	{
		ui := cli.NewMockUi()
		c := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"foo", "bar",
		}
		require.Equal(t, 0, c.Run(args), ui.ErrorWriter.String())

		ixns, _, err := client.Connect().Intentions(nil)
		require.NoError(t, err)
		require.Len(t, ixns, 1)
		require.Equal(t, "foo", ixns[0].SourceName)
		require.Equal(t, "bar", ixns[0].DestinationName)
		require.Equal(t, api.IntentionActionAllow, ixns[0].Action)
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
		require.Equal(t, 1, c.Run(args), ui.ErrorWriter.String())
		require.Contains(t, ui.ErrorWriter.String(), "more than once")
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
		require.Equal(t, 0, c.Run(args), ui.ErrorWriter.String())

		ixns, _, err := client.Connect().Intentions(nil)
		require.NoError(t, err)
		require.Len(t, ixns, 1)
		require.Equal(t, "foo", ixns[0].SourceName)
		require.Equal(t, "bar", ixns[0].DestinationName)
		require.Equal(t, api.IntentionActionDeny, ixns[0].Action)
	}
}
