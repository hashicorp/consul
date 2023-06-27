// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package set

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
)

func TestOperatorAutopilotSetConfigCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestOperatorAutopilotSetConfigCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	c := New(ui)
	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-cleanup-dead-servers=false",
		"-max-trailing-logs=99",
		"-last-contact-threshold=123ms",
		"-server-stabilization-time=123ms",
		"-min-quorum=3",
	}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
	output := strings.TrimSpace(ui.OutputWriter.String())
	if !strings.Contains(output, "Configuration updated") {
		t.Fatalf("bad: %s", output)
	}

	req := structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	var reply structs.AutopilotConfig
	if err := a.RPC(context.Background(), "Operator.AutopilotGetConfiguration", &req, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	if reply.CleanupDeadServers {
		t.Fatalf("bad: %#v", reply)
	}
	if reply.MaxTrailingLogs != 99 {
		t.Fatalf("bad: %#v", reply)
	}
	if reply.LastContactThreshold != 123*time.Millisecond {
		t.Fatalf("bad: %#v", reply)
	}
	if reply.ServerStabilizationTime != 123*time.Millisecond {
		t.Fatalf("bad: %#v", reply)
	}
	if reply.MinQuorum != 3 {
		t.Fatalf("bad: %#v", reply)
	}
}
