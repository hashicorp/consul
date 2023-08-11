// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package reload

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/mitchellh/cli"
)

func TestReloadCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestReloadCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()

	// Setup a dummy response to errCh to simulate a successful reload
	ui := cli.NewMockUi()
	c := New(ui)
	args := []string{"-http-addr=" + a.HTTPAddr()}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if !strings.Contains(ui.OutputWriter.String(), "reload triggered") {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}
}
