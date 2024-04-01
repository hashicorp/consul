// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package exp

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/kv/impexp"
	"github.com/mitchellh/cli"
)

func TestKVExportCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(nil).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestKVExportCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	ui := cli.NewMockUi()
	c := New(ui)

	keys := map[string]string{
		"foo/a": "a",
		"foo/b": "b",
		"foo/c": "c",
		"bar":   "d",
	}
	for k, v := range keys {
		pair := &api.KVPair{Key: k, Value: []byte(v)}
		if _, err := client.KV().Put(pair, nil); err != nil {
			t.Fatalf("err: %#v", err)
		}
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

	var exported []*impexp.Entry
	err := json.Unmarshal([]byte(output), &exported)
	if err != nil {
		t.Fatalf("bad: %d", code)
	}

	if len(exported) != 3 {
		t.Fatalf("bad: expected 3, got %d", len(exported))
	}

	for _, entry := range exported {
		if base64.StdEncoding.EncodeToString([]byte(keys[entry.Key])) != entry.Value {
			t.Fatalf("bad: expected %s, got %s", keys[entry.Key], entry.Value)
		}
	}
}
