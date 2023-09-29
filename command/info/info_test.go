// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package info

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/mitchellh/cli"
)

func TestInfoCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(nil).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestInfoCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a1 := agent.NewTestAgent(t, ``)
	defer a1.Shutdown()

	ui := cli.NewMockUi()
	cmd := New(ui)
	args := []string{"-http-addr=" + a1.HTTPAddr()}

	code := cmd.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if !strings.Contains(ui.OutputWriter.String(), "agent") {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}
}
