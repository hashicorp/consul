package command

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/mitchellh/cli"
)

func testKVGetCommand(t *testing.T) (*cli.MockUi, *KVGetCommand) {
	ui := cli.NewMockUi()
	return ui, &KVGetCommand{
		BaseCommand: BaseCommand{
			UI:    ui,
			Flags: FlagSetHTTP,
		},
	}
}

func TestKVGetCommand_implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &KVGetCommand{}
}

func TestKVGetCommand_noTabs(t *testing.T) {
	t.Parallel()
	assertNoTabs(t, new(KVGetCommand))
}

func TestKVGetCommand_Validation(t *testing.T) {
	t.Parallel()
	ui, c := testKVGetCommand(t)

	cases := map[string]struct {
		args   []string
		output string
	}{
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

func TestKVGetCommand_Run(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), nil)
	defer a.Shutdown()
	client := a.Client()

	ui, c := testKVGetCommand(t)

	pair := &api.KVPair{
		Key:   "foo",
		Value: []byte("bar"),
	}
	_, err := client.KV().Put(pair, nil)
	if err != nil {
		t.Fatalf("err: %#v", err)
	}

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"foo",
	}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	output := ui.OutputWriter.String()
	if !strings.Contains(output, "bar") {
		t.Errorf("bad: %#v", output)
	}
}

func TestKVGetCommand_Missing(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	_, c := testKVGetCommand(t)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"not-a-real-key",
	}

	code := c.Run(args)
	if code == 0 {
		t.Fatalf("expected bad code")
	}
}

func TestKVGetCommand_Empty(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), nil)
	defer a.Shutdown()
	client := a.Client()

	ui, c := testKVGetCommand(t)

	pair := &api.KVPair{
		Key:   "empty",
		Value: []byte(""),
	}
	_, err := client.KV().Put(pair, nil)
	if err != nil {
		t.Fatalf("err: %#v", err)
	}

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"empty",
	}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
}

func TestKVGetCommand_Detailed(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), nil)
	defer a.Shutdown()
	client := a.Client()

	ui, c := testKVGetCommand(t)

	pair := &api.KVPair{
		Key:   "foo",
		Value: []byte("bar"),
	}
	_, err := client.KV().Put(pair, nil)
	if err != nil {
		t.Fatalf("err: %#v", err)
	}

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-detailed",
		"foo",
	}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	output := ui.OutputWriter.String()
	for _, key := range []string{
		"CreateIndex",
		"LockIndex",
		"ModifyIndex",
		"Flags",
		"Session",
		"Value",
	} {
		if !strings.Contains(output, key) {
			t.Fatalf("bad %#v, missing %q", output, key)
		}
	}
}

func TestKVGetCommand_Keys(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), nil)
	defer a.Shutdown()
	client := a.Client()

	ui, c := testKVGetCommand(t)

	keys := []string{"foo/bar", "foo/baz", "foo/zip"}
	for _, key := range keys {
		if _, err := client.KV().Put(&api.KVPair{Key: key}, nil); err != nil {
			t.Fatalf("err: %#v", err)
		}
	}

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-keys",
		"foo/",
	}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	output := ui.OutputWriter.String()
	for _, key := range keys {
		if !strings.Contains(output, key) {
			t.Fatalf("bad %#v missing %q", output, key)
		}
	}
}

func TestKVGetCommand_Recurse(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), nil)
	defer a.Shutdown()
	client := a.Client()

	ui, c := testKVGetCommand(t)

	keys := map[string]string{
		"foo/a": "a",
		"foo/b": "b",
		"foo/c": "c",
	}
	for k, v := range keys {
		pair := &api.KVPair{Key: k, Value: []byte(v)}
		if _, err := client.KV().Put(pair, nil); err != nil {
			t.Fatalf("err: %#v", err)
		}
	}

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-recurse",
		"foo",
	}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	output := ui.OutputWriter.String()
	for key, value := range keys {
		if !strings.Contains(output, key+":"+value) {
			t.Fatalf("bad %#v missing %q", output, key)
		}
	}
}

func TestKVGetCommand_RecurseBase64(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), nil)
	defer a.Shutdown()
	client := a.Client()

	ui, c := testKVGetCommand(t)

	keys := map[string]string{
		"foo/a": "Hello World 1",
		"foo/b": "Hello World 2",
		"foo/c": "Hello World 3",
	}
	for k, v := range keys {
		pair := &api.KVPair{Key: k, Value: []byte(v)}
		if _, err := client.KV().Put(pair, nil); err != nil {
			t.Fatalf("err: %#v", err)
		}
	}

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-recurse",
		"-base64",
		"foo",
	}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	output := ui.OutputWriter.String()
	for key, value := range keys {
		if !strings.Contains(output, key+":"+base64.StdEncoding.EncodeToString([]byte(value))) {
			t.Fatalf("bad %#v missing %q", output, key)
		}
	}
}

func TestKVGetCommand_DetailedBase64(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), nil)
	defer a.Shutdown()
	client := a.Client()

	ui, c := testKVGetCommand(t)

	pair := &api.KVPair{
		Key:   "foo",
		Value: []byte("bar"),
	}
	_, err := client.KV().Put(pair, nil)
	if err != nil {
		t.Fatalf("err: %#v", err)
	}

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-detailed",
		"-base64",
		"foo",
	}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	output := ui.OutputWriter.String()
	for _, key := range []string{
		"CreateIndex",
		"LockIndex",
		"ModifyIndex",
		"Flags",
		"Session",
		"Value",
	} {
		if !strings.Contains(output, key) {
			t.Fatalf("bad %#v, missing %q", output, key)
		}
	}

	if !strings.Contains(output, base64.StdEncoding.EncodeToString([]byte("bar"))) {
		t.Fatalf("bad %#v, value is not base64 encoded", output)
	}
}
