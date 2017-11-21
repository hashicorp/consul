package set

import (
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/mitchellh/cli"
)

func TestOperatorAutopilotSetConfigCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestOperatorAutopilotSetConfigCommmand(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()

	ui := cli.NewMockUi()
	c := New(ui)
	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-cleanup-dead-servers=false",
		"-max-trailing-logs=99",
		"-last-contact-threshold=123ms",
		"-server-stabilization-time=123ms",
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
	if err := a.RPC("Operator.AutopilotGetConfiguration", &req, &reply); err != nil {
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
}
