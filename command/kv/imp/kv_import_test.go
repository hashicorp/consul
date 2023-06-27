// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package imp

import (
	"strings"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
)

func TestKVImportCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(nil).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestKVImportCommand_EmptyDir(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	const json = `[
		{
			"key": "foo/",
			"flags": 0,
			"value": ""
		}
	]`

	ui := cli.NewMockUi()
	c := New(ui)
	c.testStdin = strings.NewReader(json)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-",
	}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	pair, _, err := client.KV().Get("foo", nil)
	require.NoError(t, err)
	require.Nil(t, pair)

	pair, _, err = client.KV().Get("foo/", nil)
	require.NoError(t, err)
	require.Equal(t, "foo/", pair.Key)
}

func TestKVImportCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	const json = `[
		{
			"key": "foo",
			"flags": 0,
			"value": "YmFyCg=="
		},
		{
			"key": "foo/a",
			"flags": 0,
			"value": "YmF6Cg=="
		}
	]`

	ui := cli.NewMockUi()
	c := New(ui)
	c.testStdin = strings.NewReader(json)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-",
	}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	pair, _, err := client.KV().Get("foo", nil)
	if err != nil {
		t.Fatal(err)
	}

	if strings.TrimSpace(string(pair.Value)) != "bar" {
		t.Fatalf("bad: expected: bar, got %s", pair.Value)
	}

	pair, _, err = client.KV().Get("foo/a", nil)
	if err != nil {
		t.Fatal(err)
	}

	if strings.TrimSpace(string(pair.Value)) != "baz" {
		t.Fatalf("bad: expected: baz, got %s", pair.Value)
	}
}

func TestKVImportPrefixCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	const json = `[
		{
			"key": "foo",
			"flags": 0,
			"value": "YmFyCg=="
		}
	]`

	ui := cli.NewMockUi()
	c := New(ui)
	c.testStdin = strings.NewReader(json)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-prefix=" + "sub/",
		"-",
	}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	pair, _, err := client.KV().Get("foo", nil)
	if err != nil {
		t.Fatal(err)
	}

	if pair != nil {
		t.Fatalf("bad: expected: nil, got %+v", pair)
	}

	pair, _, err = client.KV().Get("sub/foo", nil)
	if err != nil {
		t.Fatal(err)
	}

	if strings.TrimSpace(string(pair.Value)) != "bar" {
		t.Fatalf("bad: expected: bar, got %s", pair.Value)
	}
}
