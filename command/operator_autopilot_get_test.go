package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/command/agent"
	"github.com/hashicorp/consul/command/base"
	"github.com/mitchellh/cli"
)

func TestOperator_Autopilot_Get_Implements(t *testing.T) {
	var _ cli.Command = &OperatorAutopilotGetCommand{}
}

func TestOperator_Autopilot_Get(t *testing.T) {
	a := agent.NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	ui := new(cli.MockUi)
	c := OperatorAutopilotGetCommand{
		Command: base.Command{
			UI:    ui,
			Flags: base.FlagSetHTTP,
		},
	}
	args := []string{"-http-addr=" + a.HTTPAddr()}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
	output := strings.TrimSpace(ui.OutputWriter.String())
	if !strings.Contains(output, "CleanupDeadServers = true") {
		t.Fatalf("bad: %s", output)
	}
}
