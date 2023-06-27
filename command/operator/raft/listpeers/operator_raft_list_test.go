// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package listpeers

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/agent"
)

func TestOperatorRaftListPeersCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestOperatorRaftListPeersCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()

	expected := fmt.Sprintf("%s  %s  127.0.0.1:%d  leader  true   3             1             -",
		a.Config.NodeName, a.Config.NodeID, a.Config.ServerPort)

	// Test the list-peers subcommand directly
	ui := cli.NewMockUi()
	c := New(ui)
	args := []string{"-http-addr=" + a.HTTPAddr()}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
	output := strings.TrimSpace(ui.OutputWriter.String())
	if !strings.Contains(output, expected) {
		t.Fatalf("bad: %q, %q", output, expected)
	}
}

func TestOperatorRaftListPeersCommand_verticalBar(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	nodeName := "name|with|bars"
	a := agent.NewTestAgent(t, `node_name = "`+nodeName+`"`)
	defer a.Shutdown()

	ui := cli.NewMockUi()
	c := New(ui)
	c.flags.SetOutput(ui.ErrorWriter)

	args := []string{"-http-addr=" + a.HTTPAddr()}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad exit code: %d. %q", code, ui.ErrorWriter.String())
	}

	// Check for nodeName presense because it should not be parsed by columnize
	if !strings.Contains(ui.OutputWriter.String(), nodeName) {
		t.Fatalf("expected %q to contain %q", ui.OutputWriter.String(), nodeName)
	}
}
