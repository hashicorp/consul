package del

import (
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/mitchellh/cli"
)

func TestKVDeleteCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(nil).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestKVDeleteCommand_Validation(t *testing.T) {
	t.Parallel()
	ui := cli.NewMockUi()
	c := New(ui)

	cases := map[string]struct {
		args   []string
		output string
	}{
		"-cas and -recurse": {
			[]string{"-cas", "-modify-index", "2", "-recurse", "foo"},
			"Cannot specify both",
		},
		"-cas no -modify-index": {
			[]string{"-cas", "foo"},
			"Cannot delete a key that does not exist",
		},
		"-modify-index no -cas": {
			[]string{"-modify-index", "2", "foo"},
			"Cannot specify -modify-index without",
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

func TestKVDeleteCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	ui := cli.NewMockUi()
	c := New(ui)

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

	pair, _, err = client.KV().Get("foo", nil)
	if err != nil {
		t.Fatalf("err: %#v", err)
	}
	if pair != nil {
		t.Fatalf("bad: %#v", pair)
	}
}

func TestKVDeleteCommand_Recurse(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	ui := cli.NewMockUi()
	c := New(ui)

	keys := []string{"foo/a", "foo/b", "food"}

	for _, k := range keys {
		pair := &api.KVPair{
			Key:   k,
			Value: []byte("bar"),
		}
		_, err := client.KV().Put(pair, nil)
		if err != nil {
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

	for _, k := range keys {
		pair, _, err := client.KV().Get(k, nil)
		if err != nil {
			t.Fatalf("err: %#v", err)
		}
		if pair != nil {
			t.Fatalf("bad: %#v", pair)
		}
	}
}

func TestKVDeleteCommand_CAS(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	ui := cli.NewMockUi()
	c := New(ui)

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
		"-cas",
		"-modify-index", "1",
		"foo",
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
		"foo",
	}

	code = c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	data, _, err = client.KV().Get("foo", nil)
	if err != nil {
		t.Fatal(err)
	}
	if data != nil {
		t.Fatalf("bad: %#v", data)
	}
}
