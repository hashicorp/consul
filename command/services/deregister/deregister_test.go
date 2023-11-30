// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package deregister

import (
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil"
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
		"no args or id": {
			[]string{},
			"at least one",
		},
		"args and -id": {
			[]string{"-id", "web", "foo.json"},
			"not both",
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

func TestCommand_File_id(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	// Register a service
	require.NoError(t, client.Agent().ServiceRegister(&api.AgentServiceRegistration{
		Name: "web"}))
	require.NoError(t, client.Agent().ServiceRegister(&api.AgentServiceRegistration{
		Name: "db"}))

	ui := cli.NewMockUi()
	c := New(ui)

	contents := `{ "Service": { "ID": "web", "Name": "foo" } }`
	f := testFile(t, "json")
	defer os.Remove(f.Name())
	if _, err := f.WriteString(contents); err != nil {
		t.Fatalf("err: %#v", err)
	}

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		f.Name(),
	}

	require.Equal(t, 0, c.Run(args), ui.ErrorWriter.String())

	svcs, err := client.Agent().Services()
	require.NoError(t, err)
	require.Len(t, svcs, 1)
	require.NotNil(t, svcs["db"])
}

func TestCommand_File_nameOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	// Register a service
	require.NoError(t, client.Agent().ServiceRegister(&api.AgentServiceRegistration{
		Name: "web"}))
	require.NoError(t, client.Agent().ServiceRegister(&api.AgentServiceRegistration{
		Name: "db"}))

	ui := cli.NewMockUi()
	c := New(ui)

	contents := `{ "Service": { "Name": "web" } }`
	f := testFile(t, "json")
	defer os.Remove(f.Name())
	if _, err := f.WriteString(contents); err != nil {
		t.Fatalf("err: %#v", err)
	}

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		f.Name(),
	}

	require.Equal(t, 0, c.Run(args), ui.ErrorWriter.String())

	svcs, err := client.Agent().Services()
	require.NoError(t, err)
	require.Len(t, svcs, 1)
	require.NotNil(t, svcs["db"])
}

func TestCommand_Flag(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	// Register a service
	require.NoError(t, client.Agent().ServiceRegister(&api.AgentServiceRegistration{
		Name: "web"}))
	require.NoError(t, client.Agent().ServiceRegister(&api.AgentServiceRegistration{
		Name: "db"}))

	ui := cli.NewMockUi()
	c := New(ui)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-id", "web",
	}

	require.Equal(t, 0, c.Run(args), ui.ErrorWriter.String())

	svcs, err := client.Agent().Services()
	require.NoError(t, err)
	require.Len(t, svcs, 1)
	require.NotNil(t, svcs["db"])
}

func testFile(t *testing.T, suffix string) *os.File {
	f := testutil.TempFile(t, "register-test-file")
	if err := f.Close(); err != nil {
		t.Fatalf("err: %s", err)
	}

	newName := f.Name() + "." + suffix
	if err := os.Rename(f.Name(), newName); err != nil {
		t.Fatalf("err: %s", err)
	}

	f, err := os.Create(newName)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	return f
}
