// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package put

import (
	"bytes"
	"encoding/base64"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestKVPutCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(nil).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestKVPutCommand_Validation(t *testing.T) {
	t.Parallel()
	ui := cli.NewMockUi()
	c := New(ui)

	cases := map[string]struct {
		args   []string
		output string
	}{
		"-acquire without -session": {
			[]string{"-acquire", "foo"},
			"Missing -session",
		},
		"-release without -session": {
			[]string{"-release", "foo"},
			"Missing -session",
		},
		"no key": {
			[]string{},
			"Missing KEY argument",
		},
		"extra args": {
			[]string{"foo", "bar", "baz"},
			"Too many arguments",
		},
	}

	for name, tc := range cases {
		c.init()
		// Ensure our buffer is always clear
		if ui.ErrorWriter != nil {
			ui.ErrorWriter.Reset()
		}
		if ui.OutputWriter != nil {
			ui.OutputWriter.Reset()
		}

		code := c.Run(tc.args)
		if code == 0 {
			t.Errorf("%s: expected non-zero exit", name)
		}

		output := ui.ErrorWriter.String()
		if !strings.Contains(output, tc.output) {
			t.Errorf("%s: expected %q to contain %q", name, output, tc.output)
		}
	}
}

func TestKVPutCommand(t *testing.T) {
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
		"foo", "bar",
	}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	data, _, err := client.KV().Get("foo", nil)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(data.Value, []byte("bar")) {
		t.Errorf("bad: %#v", data.Value)
	}
}

func TestKVPutCommand_EmptyDataQuoted(t *testing.T) {
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
		"foo", "",
	}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	data, _, err := client.KV().Get("foo", nil)
	if err != nil {
		t.Fatal(err)
	}

	if data.Value != nil {
		t.Errorf("bad: %#v", data.Value)
	}
}

func TestKVPutCommand_Base64(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	ui := cli.NewMockUi()
	c := New(ui)

	const encodedString = "aGVsbG8gd29ybGQK"

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-base64",
		"foo", encodedString,
	}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	data, _, err := client.KV().Get("foo", nil)
	if err != nil {
		t.Fatal(err)
	}

	expected, err := base64.StdEncoding.DecodeString(encodedString)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(data.Value, expected) {
		t.Errorf("bad: %#v, %s", data.Value, data.Value)
	}
}

func TestKVPutCommand_File(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	ui := cli.NewMockUi()
	c := New(ui)

	f := testutil.TempFile(t, "kv-put-command-file")
	if _, err := f.WriteString("bar"); err != nil {
		t.Fatalf("err: %#v", err)
	}

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"foo", "@" + f.Name(),
	}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	data, _, err := client.KV().Get("foo", nil)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(data.Value, []byte("bar")) {
		t.Errorf("bad: %#v", data.Value)
	}
}

func TestKVPutCommand_FileNoExist(t *testing.T) {
	t.Parallel()
	ui := cli.NewMockUi()
	c := New(ui)

	args := []string{
		"foo", "@/nope/definitely/not-a-real-file.txt",
	}

	code := c.Run(args)
	if code == 0 {
		t.Fatal("bad: expected error")
	}

	output := ui.ErrorWriter.String()
	if !strings.Contains(output, "Failed to read file") {
		t.Errorf("bad: %#v", output)
	}
}

func TestKVPutCommand_Stdin(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	stdinR, stdinW := io.Pipe()

	ui := cli.NewMockUi()
	c := New(ui)
	c.testStdin = stdinR

	go func() {
		stdinW.Write([]byte("bar"))
		stdinW.Close()
	}()

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"foo", "-",
	}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	data, _, err := client.KV().Get("foo", nil)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(data.Value, []byte("bar")) {
		t.Errorf("bad: %#v", data.Value)
	}
}

func TestKVPutCommand_NegativeVal(t *testing.T) {
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
		"foo", "-2",
	}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	data, _, err := client.KV().Get("foo", nil)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(data.Value, []byte("-2")) {
		t.Errorf("bad: %#v", data.Value)
	}
}

func TestKVPutCommand_Flags(t *testing.T) {
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
		"-flags", "12345",
		"foo",
	}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	data, _, err := client.KV().Get("foo", nil)
	if err != nil {
		t.Fatal(err)
	}

	if data.Flags != 12345 {
		t.Errorf("bad: %#v", data.Flags)
	}
}

func TestKVPutCommand_CAS(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	// Create the initial pair so it has a ModifyIndex.
	pair := &api.KVPair{
		Key:   "foo",
		Value: []byte("bar"),
	}
	if _, err := client.KV().Put(pair, nil); err != nil {
		t.Fatalf("err: %#v", err)
	}

	ui := cli.NewMockUi()
	c := New(ui)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-cas",
		"-modify-index", "123",
		"foo", "a",
	}

	code := c.Run(args)
	if code == 0 {
		t.Fatalf("bad: expected error")
	}

	data, _, err := client.KV().Get("foo", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Reset buffers
	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	args = []string{
		"-http-addr=" + a.HTTPAddr(),
		"-cas",
		"-modify-index", strconv.FormatUint(data.ModifyIndex, 10),
		"foo", "a",
	}

	code = c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	data, _, err = client.KV().Get("foo", nil)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(data.Value, []byte("a")) {
		t.Errorf("bad: %#v", data.Value)
	}
}
