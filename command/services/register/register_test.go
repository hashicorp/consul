// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package register

import (
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
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
		"args and -name": {
			[]string{"-name", "web", "foo.json"},
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

func TestCommand_File(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

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

	svc := svcs["web"]
	require.NotNil(t, svc)
}

func TestCommand_Flags(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	ui := cli.NewMockUi()
	c := New(ui)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-name", "web",
	}

	require.Equal(t, 0, c.Run(args), ui.ErrorWriter.String())

	svcs, err := client.Agent().Services()
	require.NoError(t, err)
	require.Len(t, svcs, 1)

	svc := svcs["web"]
	require.NotNil(t, svc)
}

func TestCommand_Flags_TaggedAddresses(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	ui := cli.NewMockUi()
	c := New(ui)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-name", "web",
		"-tagged-address", "lan=127.0.0.1:1234",
		"-tagged-address", "v6=[2001:db8::12]:1234",
	}

	require.Equal(t, 0, c.Run(args), ui.ErrorWriter.String())

	svcs, err := client.Agent().Services()
	require.NoError(t, err)
	require.Len(t, svcs, 1)

	svc := svcs["web"]
	require.NotNil(t, svc)
	require.Len(t, svc.TaggedAddresses, 2)
	require.Contains(t, svc.TaggedAddresses, "lan")
	require.Contains(t, svc.TaggedAddresses, "v6")
	require.Equal(t, svc.TaggedAddresses["lan"].Address, "127.0.0.1")
	require.Equal(t, svc.TaggedAddresses["lan"].Port, 1234)
	require.Equal(t, svc.TaggedAddresses["v6"].Address, "2001:db8::12")
	require.Equal(t, svc.TaggedAddresses["v6"].Port, 1234)
}

func TestCommand_FileWithUnnamedCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	ui := cli.NewMockUi()
	c := New(ui)

	contents := `{ "Service": { "Name": "web", "Check": { "TTL": "10s" } } }`
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

	svc := svcs["web"]
	require.NotNil(t, svc)

	checks, err := client.Agent().Checks()
	require.NoError(t, err)
	require.Len(t, checks, 1)
}

func testFile(t *testing.T, suffix string) *os.File {
	f := testutil.TempFile(t, "register-test-file")
	if err := f.Close(); err != nil {
		t.Fatalf("err: %s", err)
	}

	newName := f.Name() + "." + suffix
	if err := os.Rename(f.Name(), newName); err != nil {
		os.Remove(f.Name())
		t.Fatalf("err: %s", err)
	}

	f, err := os.Create(newName)
	if err != nil {
		os.Remove(newName)
		t.Fatalf("err: %s", err)
	}

	return f
}
