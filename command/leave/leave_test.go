// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package leave

import (
	"strings"
	"testing"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/agent"
)

func TestLeaveCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(nil).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestLeaveCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()

	ui := cli.NewMockUi()
	c := New(ui)
	args := []string{"-http-addr=" + a.HTTPAddr()}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if !strings.Contains(ui.OutputWriter.String(), "leave complete") {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}
}

func TestLeaveCommand_FailOnNonFlagArgs(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()

	ui := cli.NewMockUi()
	c := New(ui)
	args := []string{"-http-addr=" + a.HTTPAddr(), "appserver1"}

	code := c.Run(args)
	if code == 0 {
		t.Fatalf("bad: failed to check for unexpected args")
	}
}
